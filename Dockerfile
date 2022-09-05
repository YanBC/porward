FROM golang:1.17 as build

COPY . /porward

RUN cd /porward/cmds/forward && \
go build

FROM ubuntu:18.04

COPY --from=build /porward/cmds/forward/forward /bin
