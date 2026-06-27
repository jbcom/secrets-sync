// Command secrets-sync-lambda is the AWS Lambda entrypoint. It is a thin
// adapter over pkg/serverless: Lambda decodes the event into a serverless
// Request, Handle runs the pipeline, and the Response is returned as the Lambda
// result.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jbcom/secrets-sync/pkg/serverless"
)

func main() {
	lambda.Start(handle)
}

func handle(ctx context.Context, event serverless.Request) (serverless.Response, error) {
	return serverless.Handle(ctx, event), nil
}
