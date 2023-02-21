FROM debian:stretch-slim

# based on architecture
ARG build_arch
COPY tpu-device-plugin-${build_arch} /usr/bin/tpu-device-plugin

CMD ["tpu-device-plugin"]