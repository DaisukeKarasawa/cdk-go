package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigateway"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"

	// "github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type CdkGoStackProps struct {
	awscdk.StackProps
}

func NewCdkGoStack(scope constructs.Construct, id string, props *CdkGoStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	// S3: 記事格納用
	bucket := awss3.NewBucket(stack, jsii.String("BlogPosts"), &awss3.BucketProps{})

	// Lambda: 事前にビルドしたZIPアセットを使用
	fn := awslambda.NewFunction(stack, jsii.String("BlogApi"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("dist/lambda/blog.zip"), nil),
		Environment: &map[string]*string{
			"POSTS_BUCKET": bucket.BucketName(),
		},
	})
	bucket.GrantReadWrite(fn, nil)

	// API Gateway: /posts, /posts/{id}
	api := awsapigateway.NewLambdaRestApi(stack, jsii.String("BlogApiGateway"), &awsapigateway.LambdaRestApiProps{
		Handler: fn,
	})
	_ = api

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewCdkGoStack(app, "CdkGoStack", &CdkGoStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	// If unspecified, this stack will be "environment-agnostic".
	// Account/Region-dependent features and context lookups will not work, but a
	// single synthesized template can be deployed anywhere.
	//---------------------------------------------------------------------------
	return nil

	// Uncomment if you know exactly what account and region you want to deploy
	// the stack to. This is the recommendation for production stacks.
	//--------------------------------------------------------------------------
	// return &awscdk.Environment{
	// 	 Account: jsii.String("123456789012"),
	// 	 Region:  jsii.String("us-east-1"),
	// }

	// Uncomment to specialize this stack for the AWS Account and Region that are
	// implied by the current CLI configuration. This is recommended for dev
	// stacks.
	//--------------------------------------------------------------------------
	// return &awscdk.Environment{
	// 	 Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
	// 	 Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	// }
}
