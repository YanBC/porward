# Porward

A handy **Por**t for**ward**ing tool for docker.

Publishing ports for a running docker container is not possible at the moment (Well, to be fair, it is not impossible but requires multiple commands with `docker-proxy` and `iptables` and superuser access). If you find that frustrating, this tool might help.

## How does it work?
todo: topology here


## Get forward server image
```bash
# build from source
docker build -t yanbc/porward:latest .
# or if you prefer, pull from docker.io
docker pull yanbc/porward:latest
```


## Compile `porward` command line tool
```bash
# build the tool
git clone https://github.com/YanBC/porward.git porward && \
cd porward/cmds/porward && \
go build -o porward

# print help
./porward -h

# example usage
#   Say you started a redis container named test-redis.
#   But forget to publish port 6379 and do not want to start over.
#   To publish redis port 6379 to host port 8899, run this:
./porward -c test-redis -p 8899:6379
```

## Todos:
1. Add network topology
2. Add tests
3. github actions for container build and testing
