package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/sirupsen/logrus"
)

type Markdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type OapiRobotSendRequest struct {
	MsgType  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown,omitempty"`
	At       At       `json:"at,omitempty"`
}

type At struct {
	AtMobiles []string `json:"atMobiles,omitempty"`
	AtUserIds []string `json:"atUserIds,omitempty"`
	IsAtAll   string   `json:"isAtAll,omitempty"`
}

type OapiRobotSendResponse struct {
	Errcode int64  `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

type HealthEvent struct {
	Version    string   `json:"version,omitempty"`
	ID         string   `json:"id,omitempty"`
	DetailType string   `json:"detail-type,omitempty"`
	Source     string   `json:"source,omitempty"`
	Account    string   `json:"account,omitempty"`
	Time       string   `json:"time,omitempty"`
	Region     string   `json:"region,omitempty"`
	Resources  []string `json:"resources,omitempty"`
	Detail     struct {
		Arn               string `json:"arn,omitempty"`
		Service           string `json:"service,omitempty"`
		EventTypeCode     string `json:"eventTypeCode,omitempty"`
		EventTypeCategory string `json:"eventTypeCategory,omitempty"`
		Region            string `json:"region,omitempty"`
		StartTime         string `json:"startTime,omitempty"`
		EndTime           string `json:"endTime,omitempty"`
		LastUpdatedTime   string `json:"lastUpdatedTime,omitempty"`
		StatusCode        string `json:"statusCode,omitempty"`
		EventScopeCode    string `json:"eventScopeCode,omitempty"`
	} `json:"detail,omitempty"`
}

func formatMarkdown(healthevent HealthEvent) string {
	var buffer strings.Builder

	buffer.WriteString("AWS 健康事件通知\t")
	buffer.WriteString("\n--------\t")
	buffer.WriteString("\n#### **事件类型:**\t")
	buffer.WriteString(healthevent.DetailType)
	buffer.WriteString("\n#### **账号:**\t")
	buffer.WriteString(healthevent.Account)
	buffer.WriteString("\n#### **时间:**\t")
	buffer.WriteString(healthevent.Time)
	buffer.WriteString("\n#### **地区:**\t")
	buffer.WriteString(healthevent.Region)
	buffer.WriteString("\n#### **资源:**\t")
	buffer.WriteString(strings.Join(healthevent.Resources, ","))
	buffer.WriteString("\n#### **ARN:**\t")
	buffer.WriteString(healthevent.Detail.Arn)
	buffer.WriteString("\n##### **具体服务:**\t")
	buffer.WriteString(healthevent.Detail.Service)
	buffer.WriteString("\n##### **具体事件类型码:**\t")
	buffer.WriteString(healthevent.Detail.EventTypeCode)
	buffer.WriteString("\n##### **具体地区:**\t")
	buffer.WriteString(healthevent.Detail.Region)
	buffer.WriteString("\n##### **开始时间:**\t")
	buffer.WriteString(healthevent.Detail.StartTime)
	buffer.WriteString("\n##### **结束时间:**\t")
	buffer.WriteString(healthevent.Detail.EndTime)
	buffer.WriteString("\n##### **最后更新时间:**\t")
	buffer.WriteString(healthevent.Detail.LastUpdatedTime)
	buffer.WriteString("\n##### **事件状态码:**\t")
	buffer.WriteString(healthevent.Detail.StatusCode)
	buffer.WriteString("\n##### **事件范围码:**\t")
	buffer.WriteString(healthevent.Detail.EventScopeCode)

	return buffer.String()
}

func GetSSMParameter(ctx context.Context, parameterName string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		logrus.Errorf("failed to load AWS config: %s", err)
	}

	svc := ssm.NewFromConfig(cfg)

	input := &ssm.GetParametersInput{
		Names:          []string{parameterName},
		WithDecryption: aws.Bool(true),
	}

	atMobilesList, err := svc.GetParameters(ctx, input)

	if err != nil {
		logrus.Errorf("failed to get parameter: %s", err)
		return "", err
	}

	if len(atMobilesList.Parameters) == 0 {
		logrus.Infof("parameter %s is not set", parameterName)
		return "", err
	}

	return *atMobilesList.Parameters[0].Value, nil
}

func GetSSMPrefix(ctx context.Context) (string, error) {
	ssmPrefix := os.Getenv("SSM_PREFIX")

	if ssmPrefix == "" {
		logrus.Errorf("SSM prifix is not set in environment variable: SSM_PRIFIX")
	}
	return ssmPrefix, nil
}

func GetSecretValue(ctx context.Context) (string, string, error) {

	secretKey := os.Getenv("BOT_SECRET_KEY")

	if secretKey == "" {
		logrus.Errorf("bot secret key is not set in environment variable: BOT_SECRET_KEY")
	}
	secretARN := os.Getenv("WEBHOOK_SECRET_ARN")

	if secretARN == "" {
		logrus.Errorf("webhook secret ARN of bot is not set in environment variable: WEBHOOK_SECRET_ARN")
	}

	cfg, err := config.LoadDefaultConfig(ctx)

	if err != nil {
		logrus.Errorf("failed to load AWS config: %s", err)
	}

	svc := secretsmanager.NewFromConfig(cfg)

	output, err := svc.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretARN),
	})

	if err != nil {
		logrus.Errorf("failed to get secret value: %s", err)
	}

	secretValue := aws.ToString(output.SecretString)
	if secretValue == "" {
		logrus.Errorf("secret value is empty")
	}
	return secretValue, secretKey, nil
}

func HandleRequest(ctx context.Context, snsEvent events.SNSEvent) error {

	defer func() {
		if r := recover(); r != nil {
			logrus.Infof("panic is %v", string(debug.Stack()))
		}
	}()

	snsMsg := snsEvent.Records[0].SNS.Message

	logrus.Infof(snsMsg)

	var healthevent HealthEvent

	err := json.Unmarshal([]byte(snsMsg), &healthevent)

	if err != nil {
		logrus.Errorf("%s", err)
	}

	secretValue, secretKey, _ := GetSecretValue(ctx)

	paraPrefix, _ := GetSSMPrefix(ctx)

	atMobilesParameter := "/" + paraPrefix + "/" + secretKey + "/AtMobiles/" + healthevent.Detail.Service

	defaultAtMobilesParameter := "/" + paraPrefix + "/" + secretKey + "/AtMobiles/DEFAULT"

	isAtAll := "false"

	atMobilesList, _ := GetSSMParameter(ctx, atMobilesParameter)
	logrus.Infof("The value of atMobilesList when service parameter exist: %s", atMobilesList)

	if atMobilesList == "" {
		atMobilesList, _ = GetSSMParameter(ctx, defaultAtMobilesParameter)
		logrus.Infof("The value of atMobilesList when service parameter does not exist but default parameter exist: %s", atMobilesList)
	}

	phoneNumbers := "@" + strings.ReplaceAll(atMobilesList, ",", " @")

	if atMobilesList == "" {
		isAtAll = "true"
		phoneNumbers = ""
	}

	markdownText := formatMarkdown(healthevent) + "\n-------- \n ##### **事件联系人:** \t\n" + phoneNumbers
	req := OapiRobotSendRequest{
		MsgType: "markdown",
		Markdown: Markdown{
			Title: "AWS健康事件通知: 来自" + secretKey + "机器人",
			Text:  markdownText,
		},
		At: At{
			AtMobiles: strings.Fields(strings.ReplaceAll(atMobilesList, ",", " ")),
			IsAtAll:   isAtAll,
		},
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}
	logrus.Infof("jsonReq: %s", jsonReq)

	httpReq, err := http.NewRequest("POST", secretValue, bytes.NewBuffer(jsonReq))

	logrus.Infof("httpReq: %v", httpReq)

	if err != nil {
		return fmt.Errorf("error creating HTTP request: %v", err)
	}

	//	logrus.Infof("httpReq: %v", httpReq)

	httpReq.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	//	logrus.Infof("resp: %v", resp)

	var jsonResp OapiRobotSendResponse

	//	logrus.Infof("jsonResp: %v", jsonResp)

	err = json.NewDecoder(resp.Body).Decode(&jsonResp)

	if err != nil {
		return fmt.Errorf("error decoding JSON: %v", err)
	}

	if jsonResp.Errcode != 0 {
		return fmt.Errorf("error sending message: %d %s", jsonResp.Errcode, jsonResp.Errmsg)
	}

	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
