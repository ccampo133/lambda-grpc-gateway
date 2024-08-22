# lambda-grpc-gateway

Example project demonstrating running a gRPC-gateway server on AWS Lambda.

## Overview

This project demonstrates how to run a gRPC-gateway server on AWS Lambda and API
Gateway. The gRPC-gateway server has a single ping unary rpc that returns the
message "pong" when called. This is translated to the `GET /v1/ping` HTTP
endpoint by gRPC-gateway, and it is this endpoint that is exposed by AWS API
Gateway. Additionally, the gRPC server has a single unary interceptor which logs
all the incoming gRPC requests.

Note that the gRPC service rpcs _are not accessible_ via API Gateway - only the
HTTP proxy endpoints are. However, we can exploit a clever trick to run the gRPC
_server_ on Lambda and expose the HTTP proxy endpoints via API Gateway.

According to the [gRPC-gateway documentation][gwdoc], it is architected as
follows:

<img src="doc/grpc_gw_arch.svg" width=75% height=75%>

This means generally that it runs two servers - one for gRPC and one for HTTP.
The HTTP server is a reverse proxy that translates HTTP requests to gRPC calls,
and actually makes the gRPC calls to the gRPC server. The servers listen on
different ports typically (although you can configure them to listen on the same
port using something like [cmux][cmux])

Immediately you can see that this architecture is not directly compatible with
AWS Lambda, which is stateless by definition and does not support the concept of
running servers and listening on any port. Lambda provides an
[adapter library][lib] to support HTTP APIs via standard Go
[`http.Handler`][handler]s, but it doesn't support running a server. In fact, if
you try to listen with a [`net.Listener`][listen] in a Lambda function, you will
get an error.

Luckily, an [`http.Handler`][handler] is all we need to run the gRPC-gateway
reverse proxy component on Lambda, and we can use gRPC-gateway's
[`runtime.ServeMux`][mux] for that. However, we're still left with the problem
of running the gRPC server which the proxy makes outbound requests to. As
I just mentioned, Lambda forbids running servers. So how then, can gRPC-gateway
work?

This is where the trick comes in. We actually _can_ run the gRPC server on
Lambda, but we can't listen on a port. Instead, we can create a Unix domain
socket and listen on that! You might point out that Lambda's filesystem is
[readonly][ro], and you would generally be right... but luckily, the `/tmp`
directory is writable (up to [512 MB][limit] by default).

The plan of attack is then as follows:

1. Create a Unix domain socket in `/tmp` and create a `net.Listener` to listen
   on it.
2. Run the gRPC server using this listener.
3. Create the gRPC-Gateway HTTP reverse proxy, and configure its gRPC
   entrypoint to be the gRPC server's Unix domain socket (see [docs][gwentry]).
4. Pass the HTTP reverse proxy's [`runtime.ServeMux`][mux] to the Lambda
   [adapter library][lib].
5. Run the Lambda function with the handler function.

<p align="center">
   <img alt="architecture" src="doc/arch.svg" width=75% height=75%>
</p>

This ends up working surprisingly well, and I haven't encountered any issues
yet. All the interceptors and other gRPC server features work as expected. The
main downside is that the gRPC server is not exposed via Lambda and API Gateway,
but this is a limitation of those platforms, not this project.

## FAQ

### Can I make gRPC requests to the Lambda function?

Nope.

### Can't I just use the gRPC-gateway mux without the gRPC server?

Yes and no. If you register your services directly to the mux using the
`Register...Handler` methods (e.g. [`RegisterPingServiceHandler`][reg] in this
example project), then you can use the mux without a gRPC server. However, if
you want to use gRPC specific features like interceptors (commonly used for auth
for example), which are registered directly to the gRPC
[`Server`](https://pkg.go.dev/google.golang.org/grpc#Server), you'll be out of
luck. The use of interceptors was actually the main motivation for this project.

### Just... why?

Good question. As I just mentioned, you still can't make gRPC requests to the
Lambda function. So why not just use a regular [`http.Handler`][handler]?

If that's all you need, then you're right - you don't need to go through all
this trouble. However, if you already have a gRPC service that you want to run
on Lambda, and you want to expose it via HTTP, then this is a good way to do it.

If you're starting from scratch, I would recommend using [Connect](connect). You
should be able to pass regular HTTP multiplexer to the Lambda adapter and be
done with it.

## Usage

You can run this example project as a command line application or as a Lambda
function. Obviously the Lambda function is the more interesting part, but the
command line application is useful for playing around.

The API is defined by the `.proto` files in the [`proto`](proto). Currently,
this example has a single "ping" rpc that returns the message "pong" when
called. This is defined in [`ping.proto`](proto/example/v1/ping.proto). The
corresponding HTTP endpoint is `GET /v1/ping` and it returns the same message.
The API spec is defined in the OpenAPI v2 spec
[`ping.swagger.json`](spec/example/v1/ping.swagger.json).

### Command Line

To run the command line application, simply run the following:

```sh
go run cmd/cli/main.go
```

By default, this will start an HTTP server on `[::]:8080` and a gRPC server
on `[::]:8081`. You can change the listen addresses via command line flags. Use
the `--help` flag to see the available options.

You can then make requests to the gRPC-gateway server. For example:

```sh
# Using grpcurl
grpcurl -plaintext localhost:8081 example.v1.PingService/Ping
# Output:
# {
#  "message": "Pong"
# }

# Using curl
curl http://localhost:8080/v1/ping
# Output:
# {"message":"Pong"}
```

After making some requests, check the log output. You should see the incoming
gRPC requests logged, e.g.:

```
{"level":"info","msg":"Application server started","time":"2024-08-20T18:25:10-04:00"}
{"level":"info","msg":"gRPC server listening at [::]:8081","time":"2024-08-20T18:25:10-04:00"}
{"level":"info","msg":"HTTP server listening at [::]:8080","time":"2024-08-20T18:25:10-04:00"}
{"level":"debug","msg":"rpc call: method = /example.v1.PingService/Ping","time":"2024-08-20T18:25:16-04:00"}
{"level":"debug","msg":"rpc call: method = /example.v1.PingService/Ping","time":"2024-08-20T18:25:43-04:00"}
```

### Lambda

To run the Lambda function, you will need to deploy it to AWS. You can do this
using the provided Terraform configuration in [`terraform`](terraform).

First you need to build the Go binary and zip it up. There is a
[Makefile](Makefile) target for this:

```sh
make build-lambda-zip
```

Then, from the `terraform` directory, you can deploy the Lambda function. It
requires AWS credentials, e.g. as environment variables in this example:

```sh
export AWS_PROFILE=my-profile
terraform init
terraform apply
```

This will deploy the Lambda function and API Gateway. The API Gateway endpoint
will be printed at the end of the `terraform apply` output. You can use this
endpoint to make HTTP requests to the Lambda function.

The usage is the same as the command line application, to the API Gateway URL:

```sh
# From the terraform directory
curl $(terraform output -raw endpoint)/v1/ping
# Output:
# {"message":"Pong"}
```

The Lambda logs can be viewed in the AWS console via CloudWatch.

## Project Layout and Development

This is a standard gRPC-gateway Go project. The API spec is defined by the
`.proto` files in the [`proto`](proto) directory, and are built using
[Buf](https://buf.build). The [Makefile](Makefile) has a `generate` target for
generating the Go code from the `.proto` files, which is output to the
[`gen/go`](gen/go) directory. See [`buf.gen.yaml`](buf.gen.yaml) and
[`buf.yaml`](buf.yaml) for the Buf configuration.

Feel free to explore the code and modify it as you see fit. The Lambda
entrypoint is in [`cmd/lambda/main.go`](cmd/lambda/main.go), and the CLI
entrypoint is in [`cmd/cli/main.go`](cmd/cli/main.go). Please read the code
comments for more information.

[gwdoc]: https://github.com/grpc-ecosystem/grpc-gateway/blob/main/README.md

[cmux]: https://github.com/soheilhy/cmux

[lib]: https://github.com/awslabs/aws-lambda-go-api-proxy

[listen]: https://pkg.go.dev/net#Listener

[handler]: https://pkg.go.dev/net/http#Handler

[mux]: https://pkg.go.dev/github.com/grpc-ecosystem/grpc-gateway/runtime#ServeMux

[ro]: https://stackoverflow.com/q/39383465

[limit]: https://docs.aws.amazon.com/lambda/latest/dg/gettingstarted-limits.html

[gwentry]: https://github.com/grpc-ecosystem/grpc-gateway?tab=readme-ov-file#5-write-an-entrypoint-for-the-http-reverse-proxy-server

[connect]: https://connectrpc.com/

[reg]: gen/go/example/v1/ping.pb.gw.go
