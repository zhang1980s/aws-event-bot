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
	"github.com/sirupsen/logrus"
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
	stackSetGroupName := stack.Node().TryGetContext(jsii.String("groupName")).(string)

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

	dingTalkEventTopic := awssns.NewTopic(stack, jsii.String("EventTopicMAD"+stackSetGroupName), &awssns.TopicProps{
		DisplayName: jsii.String("MultiAccountDingTalkEventTopic" + stackSetGroupName),
	})

	healthEventBus := awsevents.NewEventBus(stack, jsii.String("EventBusMAD"+stackSetGroupName), &awsevents.EventBusProps{
		EventBusName: jsii.String("MultiAccountHealthEventBus" + stackSetGroupName),
	})

	busRuleStatement := awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:     awsiam.Effect_ALLOW,
		Principals: &[]awsiam.IPrincipal{awsiam.NewAnyPrincipal()},
		Actions:    &[]*string{jsii.String("events:PutEvents")},
		Resources:  &[]*string{jsii.String(*healthEventBus.EventBusArn())},
		Sid:        jsii.String("MultiAccountHealthEventBusPolicy-" + stackSetGroupName),
		//		Conditions: map[string]interface{}{
		//			"StringLike": map[string]string{
		//				"aws:SourceArn": "HealthEventBus",
		//			},
		//		},
	})

	healthEventBus.AddToResourcePolicy(busRuleStatement)

	healthEventRule := awsevents.NewRule(stack, jsii.String("HealthEventRuleMAD"+stackSetGroupName), &awsevents.RuleProps{
		Description:  jsii.String("Multi-Account Health Event Notification Rule"),
		EventPattern: &awsevents.EventPattern{DetailType: &[]*string{jsii.String("AWS Health Event"), jsii.String("CUSTOM")}, Source: &[]*string{jsii.String("aws.health"), jsii.String("custom.dingtalkevent.test")}},
		Enabled:      jsii.Bool(true),
		EventBus:     healthEventBus,
	})

	// Secret Manager Secret to store the webhook endpoint

	dingTalkCustomBotSecret := awssecretsmanager.NewSecret(stack, jsii.String("BotSecretMAD"+stackSetGroupName), &awssecretsmanager.SecretProps{
		Description:       jsii.String("Secret to store the endpoint"),
		SecretStringValue: awscdk.SecretValue_CfnParameter(webHook),
	})

	// DingTalk CustomBot Lambda

	dingTalkCustomBotHandler := awslambda.NewFunction(stack, jsii.String("BotHandlerMAD"+stackSetGroupName), &awslambda.FunctionProps{
		Code:       awslambda.Code_FromAsset(jsii.String("eventHandler"), nil),
		Handler:    jsii.String("bin/main"),
		Runtime:    awslambda.Runtime_GO_1_X(),
		MemorySize: jsii.Number(128),
		Timeout:    awscdk.Duration_Seconds(jsii.Number(10)),
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

	dingTalkCustomBotHandler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:    awsiam.Effect_ALLOW,
		Actions:   &[]*string{jsii.String("health:DescribeEventDetailsForOrganization")},
		Resources: &[]*string{jsii.String("*")},
	}))

	dingTalkCustomBotHandler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Effect:    awsiam.Effect_ALLOW,
		Actions:   &[]*string{jsii.String("organizations:ListAccounts")},
		Resources: &[]*string{jsii.String("*")},
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
        - aws.health
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
      - PolicyName: PutEventsDestinationBus-CONTEXTGROUPNAME
        PolicyDocument:
          Version: '2012-10-17'
          Statement:
          - Effect: Allow
            Action:
            - events:PutEvents
            Resource:
            - "EVENTBUSARN"`

	r := strings.NewReplacer(
		"EVENTBUSARN", *healthEventBus.EventBusArn(),
		"CONTEXTGROUPNAME", stackSetGroupName,
	)

	awscdk.NewCfnStackSet(stack, jsii.String("EventRuleStackSet-MAD-"+stackSetGroupName), &awscdk.CfnStackSetProps{
		PermissionModel: jsii.String("SERVICE_MANAGED"),
		StackSetName:    jsii.String("EventRuleStackSet-MAD-" + stackSetGroupName),
		AutoDeployment: &awscdk.CfnStackSet_AutoDeploymentProperty{
			Enabled:                      jsii.Bool(true),
			RetainStacksOnAccountRemoval: jsii.Bool(true),
		},
		TemplateBody: jsii.String(r.Replace(yaml)),
		Capabilities: jsii.Strings("CAPABILITY_NAMED_IAM"),
	})

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

	NewDingTalkEventBotStack(app, "EventBotStack-MAD-"+groupName, &DingTalkEventBotStackProps{
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
