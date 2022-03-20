# mackerel-plugin-aws-direct-connect

```
$ mackerel-plugin-aws-direct-connect -help
Usage of mackerel-plugin-aws-direct-connect:
  -access-key-id string
        AWS Access Key ID
  -direct-connect-connection string
        Resource ID of Direct Connect
  -metric-key-prefix string
        Metric Key Prefix
  -region string
        AWS Region
  -role-arn string
        IAM Role ARN for assume role
  -secret-key-id string
        AWS Secret Access Key ID
$
```

## use Assume Role

create IAM Role with the AWS Account that created Transit Gateway Attachment.

- no MFA
- allowed Policy
    - CloudWatchReadOnlyAccess

create IAM Policy with the AWS Account that runs mackerel-plugin-aws-transitgateway-attachment.

```json
{
    "Version": "2012-10-17",
    "Statement": {
        "Effect": "Allow",
        "Action": "sts:AssumeRole",
        "Resource": "arn:aws:iam::123456789012:role/YourIAMRoleName"
    }
}
```

attach IAM Policy to AWS Resouce that runs mackerel-plugin-aws-transitgateway-attachment.

## Synopsis

use assume role.
```shell
mackerel-plugin-aws-direct-connect -role-arn <IAM Role Arn> -region <region> \
                                   -direct-connect-connection <Resource ID of Direct Connect>
```

use access key id and secret key.
```shell
mackerel-plugin-aws-direct-connect -region <region> \
                                   -direct-connect-connection <Resource ID of Direct Connect>
                                  [-access-key-id <AWS Access Key ID> -secret-key-id <WS Secret Access Key ID>] \
```
