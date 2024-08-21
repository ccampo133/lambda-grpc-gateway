# lambda-grpc-gateway

Example project demonstrating running a gRPC-gateway server on AWS Lambda.

# Overview

This project demonstrates how to run a gRPC-gateway server on AWS Lambda and API
Gateway. The gRPC-gateway server has a single ping rpc that returns the message
"pong" when called. This is translated to the `GET /v1/ping` endpoint by
gRPC-gateway, and it is this endpoint that is exposed by AWS API Gateway.

Note that the gRPC endpoints _are not accessible_ via API Gateway - only the
HTTP proxy endpoints are. However, this project exploits a clever trick to
run the gRPC server on Lambda and expose the HTTP proxy endpoints via API
Gateway.

TODO: rest of the README.

# Development

TODO
