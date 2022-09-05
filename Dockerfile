FROM golang:1.17 as build

RUN git clone https://github.com/YanBC/porward.git /porward && \
cd /porward/cmds/forward && \
go build

FROM busybox:1.34.1 

COPY --from=build /porward/cmds/forward/forward /bin
