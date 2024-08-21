module "lambda_function" {
  source = "terraform-aws-modules/lambda/aws"

  function_name = "lambda-grpc-gateway"
  description   = "gRPC Gateway Lambda function"
  runtime       = "provided.al2023"
  handler       = "bootstrap"

  create_package         = false
  publish                = true
  local_existing_package = "../lambda.zip"

  allowed_triggers = {
    APIGateway = {
      service    = "apigateway"
      source_arn = "${module.api_gateway.api_execution_arn}/*/*/*"
    }
  }
}

module "api_gateway" {
  source = "terraform-aws-modules/apigateway-v2/aws"

  name          = "lambda-grpc-gateway-api"
  description   = "HTTP API gateway for the gRPC Gateway Lambda function"
  protocol_type = "HTTP"

  create_domain_name    = false
  create_domain_records = false
  create_certificate    = false

  routes = {
    "ANY /{proxy+}" = {
      integration = {
        uri                    = module.lambda_function.lambda_function_invoke_arn
        payload_format_version = "1.0"
        timeout_milliseconds   = 30000
      }
    }
  }
}

output "endpoint" {
  value = "${module.api_gateway.api_endpoint}/v1/ping"
}
