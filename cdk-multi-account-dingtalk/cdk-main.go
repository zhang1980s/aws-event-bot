package main

import (
	"os"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/aws-cdk-go/awscdk/v2/awseventstargets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssecretsmanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssns"
	"github.com/aws/aws-cdk-go/awscdk/v2/awssnssubscriptions"
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

	//Resources:

	dingTalkEventTopic := awssns.NewTopic(stack, jsii.String("DingTalkEventTopic"), &awssns.TopicProps{
		DisplayName: jsii.String("DingTalkEventTopic"),
	})

	healthEventBus := awsevents.NewEventBus(stack, jsii.String("HealthEventBus"), &awsevents.EventBusProps{
		EventBusName: jsii.String("HealthEventBus"),
	})

	busRuleStatement := awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:     awsiam.Effect_ALLOW,
		Principals: &[]awsiam.IPrincipal{awsiam.NewAnyPrincipal()},
		Actions:    &[]*string{jsii.String("events:PutEvents")},
		Resources:  &[]*string{jsii.String(*healthEventBus.EventBusArn())},
		Sid:        jsii.String("HealthEventBusPolicy"),
		//		Conditions: map[string]interface{}{
		//			"StringLike": map[string]string{
		//				"aws:SourceArn": "HealthEventBus",
		//			},
		//		},
	})

	healthEventBus.AddToResourcePolicy(busRuleStatement)

	healthEventRule := awsevents.NewRule(stack, jsii.String("HealthEventRule"), &awsevents.RuleProps{
		Description:  jsii.String("Health Event Notification Rule"),
		EventPattern: &awsevents.EventPattern{DetailType: &[]*string{jsii.String("AWS Health Event"), jsii.String("CUSTOM")}, Source: &[]*string{jsii.String("aws.health"), jsii.String("custom.dingtalkevent.test")}},
		Enabled:      jsii.Bool(true),
		EventBus:     healthEventBus,
	})

	// Secret Manager Secret to store the webhook endpoint

	dingTalkCustomBotSecret := awssecretsmanager.NewSecret(stack, jsii.String("DingTalkCustomBotSecret"), &awssecretsmanager.SecretProps{
		Description:       jsii.String("Secret to store the endpoint"),
		SecretStringValue: awscdk.SecretValue_CfnParameter(webHook),
	})

	// DingTalk CustomBot Lambda

	dingTalkCustomBotHandler := awslambda.NewFunction(stack, jsii.String("DingTalkCustomBotHandler"), &awslambda.FunctionProps{
		Code:       awslambda.Code_FromAsset(jsii.String("eventHandler"), nil),
		Handler:    jsii.String("bin/main"),
		Runtime:    awslambda.Runtime_GO_1_X(),
		MemorySize: jsii.Number(128),
		Timeout:    awscdk.Duration_Seconds(jsii.Number(10)),
	})

	dingTalkCustomBotHandler.AddEnvironment(jsii.String("BOT_SECRET_KEY"), botSecretKey.ValueAsString(), nil)

	dingTalkCustomBotHandler.AddEnvironment(jsii.String("WEBHOOK_SECRET_ARN"), dingTalkCustomBotSecret.SecretArn(), nil)

	dingTalkCustomBotHandler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:    awsiam.Effect_ALLOW,
		Actions:   &[]*string{jsii.String("secretsmanager:GetSecretValue")},
		Resources: &[]*string{dingTalkCustomBotSecret.SecretArn()},
	}))

	// Event Bridge Rule to trigger the DingTalk CustomBot Lambda

	healthEventRule.AddTarget(awseventstargets.NewSnsTopic(dingTalkEventTopic, nil))

	dingTalkEventTopic.AddSubscription(awssnssubscriptions.NewLambdaSubscription(dingTalkCustomBotHandler, nil))

	yaml := `---
AWSTemplateFormatVersion: 2010-09-09
Description: EventBridgeRule
Resources:
  EventBridgeRule:
    Type: AWS::Events::Rule
    Properties:
      Description: Forward event to HealthEventBus
      EventBusName: default
      State: ENABLED
      EventPattern:
        source:
        - custom.dingtalkevent.test
      Targets:
      - Arn: "EVENTBUSARN"
        Id: dingtalk-eventbus-account
        RoleArn:
          Fn::GetAtt:
          - EventBridgeIAMrole
          - Arn
  EventBridgeIAMrole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
        - Effect: Allow
          Principal:
            Service:
              Fn::Sub: events.amazonaws.com
          Action: sts:AssumeRole
      Path: "/"
      Policies:
      - PolicyName: PutEventsDestinationBus
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
          - Effect: Allow
            Action:
            - events:PutEvents
            Resource:
            - "EVENTBUSARN"`

	awscdk.NewCfnStackSet(stack, jsii.String("HealthEventRuleStackSet"), &awscdk.CfnStackSetProps{
		PermissionModel: jsii.String("SERVICE_MANAGED"),
		StackSetName:    jsii.String("HealthEventRuleStackSet"),
		AutoDeployment: &awscdk.CfnStackSet_AutoDeploymentProperty{
			Enabled:                      jsii.Bool(true),
			RetainStacksOnAccountRemoval: jsii.Bool(true),
		},
		TemplateBody: jsii.String(strings.Replace(yaml, "EVENTBUSARN", *healthEventBus.EventBusArn(), -1)),
		Capabilities: jsii.Strings("CAPABILITY_NAMED_IAM"),
	})

	awscdk.NewCfnOutput(stack, jsii.String("SNSArn"), &awscdk.CfnOutputProps{
		Value: dingTalkEventTopic.TopicArn(),
	})

	return stack

}

func main() {
	app := awscdk.NewApp(nil)

	NewDingTalkEventBotStack(app, "DingTalkEventBotStack", &DingTalkEventBotStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}
