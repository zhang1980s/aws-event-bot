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
	"github.com/sirupsen/logrus"
)

type Markdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type OapiRobotSendRequest struct {
	MsgType  string   `json:"msgtype"`
	Markdown Markdown `json:"markdown,omitempty"`
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
		logrus.Errorf("failed to load AWS config: %w", err)
	}

	svc := secretsmanager.NewFromConfig(cfg)

	output, err := svc.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretARN),
	})

	if err != nil {
		logrus.Errorf("failed to get secret value: %w", err)
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
		logrus.Errorf("%w", err)
	}

	secretValue, secretKey, _ := GetSecretValue(ctx)

	resourcelist := strings.Join(healthevent.Resources, ",")

	req := OapiRobotSendRequest{
		MsgType: "markdown",
		Markdown: Markdown{
			Title: "机器人事件:" + secretKey,
			Text:  "# AWS 健康事件通知\n --- \n\n#### **事件类型:**\t" + healthevent.DetailType + "\n#### **账号:**\t" + healthevent.Account + "\n#### **时间:**\t" + healthevent.Time + "\n#### **地区:**\t" + healthevent.Region + "\n#### **资源:**\t" + resourcelist + "\n#### **ARN:**\t" + healthevent.Detail.Arn + "\n##### **具体服务:**\t" + healthevent.Detail.Service + "\n##### **具体事件类型码:**\t" + healthevent.Detail.EventTypeCode + "\n##### **具体地区:**\t" + healthevent.Detail.Region + "\n##### **开始时间:**\t" + healthevent.Detail.StartTime + "\n##### **结束时间:**\t" + healthevent.Detail.EndTime + "\n##### **最后更新时间:**\t" + healthevent.Detail.LastUpdatedTime + "\n##### **事件状态码:**\t" + healthevent.Detail.StatusCode + "\n##### **事件范围码:**\t" + healthevent.Detail.EventScopeCode,
		},
	}

	jsonReq, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}
	//	logrus.Infof("jsonReq: %s", jsonReq)
	//	logrus.Infof("webhook: %s", secretValue)

	httpReq, err := http.NewRequest("POST", secretValue, bytes.NewBuffer(jsonReq))
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
