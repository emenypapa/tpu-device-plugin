package main

import (
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const tpuNumber = 1

func main() {
	devicePlugin := NewTPUDevicePlugin(tpuNumber)
	err, _ := devicePlugin.Serve()
	if err != nil {
		log.Println("Could not contact Kubelet, Need Support!!")
	}
	// notice OS signal
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	var restartTimeout <-chan time.Time

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Printf("failed to create FS watcher: %v\n", err)
	}
	defer watcher.Close()

restart:
	log.Println("Starting Plugins.")
	err, restartTime := devicePlugin.Serve()
	if err != nil {
		log.Println("Could not contact Kubelet, Need Support!!")
	}
	if !restartTime {
		log.Printf("Failed to start one or more plugins. Retrying in 30s...")
		restartTimeout = time.After(30 * time.Second)
	}

	for {
		select {
		// If the restart timout has expired, then restart the plugins
		case <-restartTimeout:
			goto restart

		// Detect a kubelet restart by watching for a newly created
		// 'pluginapi.KubeletSocket' file. When this occurs, restart this loop,
		// restarting all of the plugins in the process.
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				goto restart
			}

		// Watch for any other fs errors and log them.
		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		// Watch for any signals from the OS. On SIGHUP, restart this loop,
		// restarting all of the plugins in the process. On all other
		// signals, exit the loop and exit the program.
		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				goto restart
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				err := devicePlugin.Stop()
				if err == nil {
					os.Exit(0)
				}
				os.Exit(255)
			}
		}
	}
}

func (t *TpuDevicePlugin) runHttpServer() {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		api.GET("/tpu", t.ImportData)
	}

	_ = r.Run(":8080")
}

type Gin struct {
	C *gin.Context
}

type Response struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
	//RequestId string      `json:"request_id"`
	Time time.Time   `json:"time"`
	Data interface{} `json:"data"`
}

func (g *Gin) Response(httpCode, errCode int32, data interface{}) {
	SetErrorCode(g.C, errCode)
	response := Response{
		Code: errCode,
		Data: data,
		Time: time.Now(),
	}
	_, err := json.Marshal(response)
	if err != nil {
		fmt.Println("json marshal error !")
	}
	//logger.Debugf(g.C, "smart_request_out: res[%s] ", string(res))
	g.C.JSON(int(httpCode), response)
	return
}

func (g *Gin) ResponseError(errCode int32, data interface{}) {
	g.Response(http.StatusOK, errCode, data)
}

func SetErrorCode(ctx *gin.Context, errorCode int32) {
	SetContextData(ctx, ErrorCode, errorCode)
}

func SetContextData(ctx *gin.Context, key string, value interface{}) {
	if ctx.Keys == nil {
		ctx.Keys = make(map[string]interface{})
	}
	//ctx.Keys[key] = value
	ctx.Set(key, value)
}

const (
	UserId    = "user_id"
	Token     = "token"
	Role      = "role_code"
	Name      = "name"
	Nickname  = "nickname"
	Avatar    = "avatar"
	Email     = "email"
	Phone     = "phone"
	ErrorCode = "error_code"
	AlarmType = "alarm_type"
	RequestId = "request_id"
	LoginType = "login_type"
)

const (
	SUCCESS                = 200   // 成功
	ERROR                  = 500   // 失败
	InvalidParams          = 400   // 参数错误
	SSONotLoggedIn         = 1100  // sso未登录
	NotLoggedIn            = 1000  // 未登录
	ParameterIllegal       = 1001  // 参数不合法
	UnauthorizedUserId     = 1002  // 用户Id不合法
	Unauthorized           = 1003  // 未授权
	ServerError            = 1004  // 系统错误
	NotData                = 1005  // 没有数据
	ModelAddError          = 1006  // 添加错误
	ModelDeleteError       = 1007  // 删除错误
	ModelStoreError        = 1008  // 存储错误
	OperationFailure       = 1009  // 操作失败
	RoutingNotExist        = 1010  // 路由不存在
	ErrorUserExist         = 1011  // 用户已存在
	ErrorUserNotExist      = 1012  // 用户不存在
	ErrorNeedCaptcha       = 1013  // 需要验证码
	PasswordInvalid        = 1014  // 密码不符合规范
	ErrorCheckTokenFail    = 10001 // 用户信息获取失败
	ErrorCheckUserRoleFail = 10002 // 用户信息获取失败
	CustomIdentityInvalid  = 10010 // 身份信息不正确
	AccessTooFrequently    = 99999 // 访问太频繁
	UnScreenshot           = 10003 // 访问太频繁
	DateTooLong            = 10005 // 日期超长
)
