#!/usr/bin/env bash
if [[ $# -ge 3 ]]; then
    export CDK_DEPLOY_ACCOUNT=$1
    export CDK_DEPLOY_REGION=$2
    shift; shift
    npx cdk "$@"
    exit $?
else
    echo 1>&2 "Provide account and region as first two args."
    echo 1>&2 "Additional args are passed through to cdk deploy."
    exit 1
fi

### Reference https://docs.aws.amazon.com/cdk/v2/guide/environments.html
### Reference https://github.com/cowcoa/aws-cdk-go-examples

### Example: 
### ./cdk-deploy-to.sh 123456789012 us-east-1 deploy --parameters ParameterKey=KeyName1,ParameterValue=MyKey1 --parameters ParameterKey=KeyName2,ParameterValue=MyKey2