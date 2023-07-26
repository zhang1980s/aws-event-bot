package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssnssubscriptions"
	"github.com/sirupsen/logrus"

	//	"github.com/aws/aws-cdk-go/awscdk/v2/awsssm"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type DingTalkEventBotStackProps struct {
	awscdk.StackProps
}

func NewDingTalkEventBotStack(scope constructs.Construct, id string, props *DingTalkEventBotStackProps) awscdk.Stack {
	var sprops awscdk.StackProps

	if props != nil {
		sprops = props.StackProps
	}

	stack := awscdk.NewStack(scope, &id, &sprops)

	//StackParameters:

	webHook := awscdk.NewCfnParameter(stack, jsii.String("WebHook"), &awscdk.CfnParameterProps{
		Description: jsii.String("The DingTalk CustomBot Webhook Endpoint"),
		Type:        jsii.String("String"),
		NoEcho:      jsii.Bool(true),
	})

	botSecretKey := awscdk.NewCfnParameter(stack, jsii.String("BotSecretKey"), &awscdk.CfnParameterProps{
		Description: jsii.String("The SecreKey of DingTalk CustomBot"),
		Type:        jsii.String("String"),
		Default:     jsii.String("AWS"),
	})

	botParaPrefix := awscdk.NewCfnParameter(stack, jsii.String("BotParaPrefix"), &awscdk.CfnParameterProps{
		Description: jsii.String("The Prefix of SSM parameter store for DingTalk CustomBot"),
		Type:        jsii.String("String"),
		Default:     jsii.String("DingtalkCustomBotParaPrefix"),
	})

	//Resources:
	// "SAD" = "Single Account DingTalk"

	dingTalkEventTopic := awssns.NewTopic(stack, jsii.String("EventTopicSAD"), &awssns.TopicProps{
		DisplayName: jsii.String("SingleAccountDingTalkEventTopic"),
	})

	healthEventRule := awsevents.NewRule(stack, jsii.String("EventRuleSAD"), &awsevents.RuleProps{
		Description:  jsii.String("Single Account Health Event Notification Rule"),
		EventPattern: &awsevents.EventPattern{DetailType: &[]*string{jsii.String("AWS Health Event"), jsii.String("CUSTOM")}, Source: &[]*string{jsii.String("aws.health"), jsii.String("custom.dingtalkevent.test")}},
		Enabled:      jsii.Bool(true),
	})

	// Secret Manager Secret to store the webhook endpoint

	dingTalkCustomBotSecret := awssecretsmanager.NewSecret(stack, jsii.String("BotSecretSAD"), &awssecretsmanager.SecretProps{
		Description:       jsii.String("Single Account Secret to store the endpoint"),
		SecretStringValue: awscdk.SecretValue_CfnParameter(webHook),
	})

	// Example Parameter in AWS System Manager Parameter store
	//	parameterName := "/" + *botParaPrefix.ValueAsString() + "/" + *botSecretKey.ValueAsString() + "/AtMobiles"

	//	awsssm.NewStringParameter(stack, jsii.String("Example parameter for DingTalk CustomBot in AWS System Manager Parameter store"), &awsssm.StringParameterProps{
	//		Description:   jsii.String("Example parameter for DingTalk CustomBot in AWS System Manager Parameter store"),
	//		ParameterName: jsii.String(parameterName),
	//		Tier:          awsssm.ParameterTier_STANDARD,
	//		StringValue:   jsii.String("123456789,987654321"),
	//	})

	// DingTalk CustomBot Lambda

	dingTalkCustomBotHandler := awslambda.NewFunction(stack, jsii.String("BotHandlerSAD"), &awslambda.FunctionProps{
		Code:       awslambda.Code_FromAsset(jsii.String("eventHandler"), nil),
		Handler:    jsii.String("bin/main"),
		Runtime:    awslambda.Runtime_GO_1_X(),
		MemorySize: jsii.Number(128),
		Timeout:    awscdk.Duration_Seconds(jsii.Number(10)),
		//		CurrentVersionOptions: &awslambda.VersionOptions{
		//			RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
		//			RetryAttempts: jsii.Number(3),
		//		},
	})

	dingTalkCustomBotHandler.AddEnvironment(jsii.String("BOT_SECRET_KEY"), botSecretKey.ValueAsString(), nil)

	dingTalkCustomBotHandler.AddEnvironment(jsii.String("WEBHOOK_SECRET_ARN"), dingTalkCustomBotSecret.SecretArn(), nil)

	dingTalkCustomBotHandler.AddEnvironment(jsii.String("SSM_PREFIX"), botParaPrefix.ValueAsString(), nil)

	dingTalkCustomBotHandler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:    awsiam.Effect_ALLOW,
		Actions:   &[]*string{jsii.String("secretsmanager:GetSecretValue")},
		Resources: &[]*string{dingTalkCustomBotSecret.SecretArn()},
	}))

	dingTalkCustomBotHandler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:    awsiam.Effect_ALLOW,
		Actions:   &[]*string{jsii.String("ssm:GetParameters")},
		Resources: &[]*string{jsii.String("arn:aws:ssm:" + *sprops.Env.Region + ":" + *sprops.Env.Account + ":parameter/" + *botParaPrefix.ValueAsString() + "/" + *botSecretKey.ValueAsString() + "/*")},
	}))

	// Event Bridge Rule to trigger the DingTalk CustomBot Lambda

	healthEventRule.AddTarget(awseventstargets.NewSnsTopic(dingTalkEventTopic, nil))

	dingTalkEventTopic.AddSubscription(awssnssubscriptions.NewLambdaSubscription(dingTalkCustomBotHandler, nil))

	// Output SNS ARN for test propose

	awscdk.NewCfnOutput(stack, jsii.String("SNSARN"), &awscdk.CfnOutputProps{
		Value: dingTalkEventTopic.TopicArn(),
	})

	return stack

}

func main() {
	app := awscdk.NewApp(nil)

	groupName, ok := app.Node().TryGetContext(jsii.String("groupName")).(string)

	if !ok || groupName == "" {
		logrus.Errorf("groupName is required")
	}

	NewDingTalkEventBotStack(app, "EventBotStack-SAD-"+groupName, &DingTalkEventBotStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	account := os.Getenv("CDK_DEPLOY_ACCOUNT")
	region := os.Getenv("CDK_DEPLOY_REGION")

	if len(account) == 0 || len(region) == 0 {
		account = os.Getenv("CDK_DEFAULT_ACCOUNT")
		region = os.Getenv("CDK_DEFAULT_REGION")
	}

	return &awscdk.Environment{
		Account: jsii.String(account),
		Region:  jsii.String(region),
	}
}
