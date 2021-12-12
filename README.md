# A Handy Port Forwarding Tool For Docker

Updating port forward configs for a running docker container is not possible at the moment (Engine 20.10.11). If you found that frustrating, this tool might help. By running a proxy server, it can forward network traffic to your specified container.

```bash
# build the tool
go build -o porward porward.go

# print help
./porward -h

# example usage
#   Say you started a redis container named test-redis.
#   But forget to publish port 6379 and do not want to start over.
#   To publish redis port 6379 to host port 8899, run this:
./porward -c test-redis -p 8899:6379
```

## How does it work?
As I said, `porward` simply starts a proxy server and forwards all network traffic from host to the specified container. This tool uses the [gost project](https://github.com/ginuerzh/gost) but hey, you can modify the codes and choose whatever proxy server you like, [nginx](https://github.com/nginx/nginx) for example.
