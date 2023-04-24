package rulestackset

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsevents"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type DingTalkEventBotRuleStack struct {
	awscdk.Stack
}

type DingTalkEventBotRuleStackProps struct {
	awscdk.StackProps
	EventBus awsevents.IEventBus
}

func NewRuleStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {

	stack := awscdk.NewStack(scope, &id, props)

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
      - Arn: "arn:aws:events:ap-southeast-1:894855526703:event-bus/HealthEventBus"
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
            - "arn:aws:events:ap-southeast-1:894855526703:event-bus/HealthEventBus"`

	awscdk.NewCfnStackSet(stack, jsii.String("MyStackSet"), &awscdk.CfnStackSetProps{
		PermissionModel: jsii.String("SERVICE_MANAGED"),
		StackSetName:    jsii.String("MyStackSet"),
		AutoDeployment: &awscdk.CfnStackSet_AutoDeploymentProperty{
			Enabled:                      jsii.Bool(true),
			RetainStacksOnAccountRemoval: jsii.Bool(true),
		},
		TemplateBody: jsii.String(yaml),
		Capabilities: jsii.Strings("CAPABILITY_NAMED_IAM"),
	})
	return stack
}
