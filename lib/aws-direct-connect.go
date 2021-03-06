package mpawsdxcon

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	mp "github.com/mackerelio/go-mackerel-plugin"
)

// AwsDxCon struct
type AwsDxCon struct {
	Prefix          string
	AccessKeyID     string
	SecretKeyID     string
	Region          string
	RoleArn         string
	DxConId         string
	FullSpecSupport bool
	CloudWatch      *cloudwatch.Client
}

const (
	namespace = "AWS/DX"
)

type metrics struct {
	Name            string
	Type            types.Statistic
	FullSpecSupport bool
}

// GraphDefinition : return graph definition
func (p AwsDxCon) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.Title(p.Prefix)
	labelPrefix = strings.Replace(labelPrefix, "-", " ", -1)

	// https://docs.aws.amazon.com/directconnect/latest/UserGuide/monitoring-cloudwatch.html#viewing-metrics
	if p.FullSpecSupport {
		return map[string]mp.Graphs{
			"State": {
				Label: labelPrefix + " connection status",
				Unit:  mp.UnitInteger,
				Metrics: []mp.Metrics{
					// The state of the connection.1 indicates up and 0 indicates down.
					{Name: "ConnectionState", Label: "ConnectionState"},
					// Indicates the connection encryption status. 1 indicates the connection encryption is up, and 0 indicates the connection encryption is down. When this metric is applied to a LAG, 1 indicates that all connections in the LAG have encrption up. 0 indicates at least one LAG connection encrption is down.
					{Name: "ConnectionEncryptionState", Label: "ConnectionEncryptionState"},
				},
			},
			"Bps": {
				Label: labelPrefix + " bps",
				Unit:  mp.UnitBitsPerSecond,
				Metrics: []mp.Metrics{
					// The bitrate for outbound data from the AWS side of the connection.
					{Name: "ConnectionBpsEgress", Label: "bps out"},
					// The bitrate for inbound data to the AWS side of the connection.
					{Name: "ConnectionBpsIngress", Label: "bps in"},
				},
			},

			"Pps": {
				Label: labelPrefix + " pps",
				Unit:  mp.UnitInteger,
				Metrics: []mp.Metrics{
					// The packet rate for outbound data from the AWS side of the connection.
					{Name: "ConnectionPpsEgress", Label: "pps out"},
					// The packet rate for inbound data to the AWS side of the connection.
					{Name: "ConnectionPpsIngress", Label: "pps in"},
				},
			},

			"LightLevel": {
				Label: labelPrefix + " Light level",
				Unit:  mp.UnitInteger,
				Metrics: []mp.Metrics{
					// Indicates the health of the fiber connection for outbound (egress) traffic from the AWS side of the connection.
					{Name: "ConnectionLightLevelTx", Label: "egress dBm"},

					// Indicates the health of the fiber connection for inbound (ingress) traffic to the AWS side of the connection.
					{Name: "ConnectionLightLevelRx", Label: "ingress dBm"},
				},
			},

			"Error": {
				Label: labelPrefix + " Error",
				Unit:  mp.UnitInteger,
				Metrics: []mp.Metrics{
					// The total error count for all types of MAC level errors on the AWS device. The total includes cyclic redundancy check (CRC) errors.
					{Name: "ConnectionErrorCount", Label: "CRC Errors"},
				},
			},
		}
	} else {
		return map[string]mp.Graphs{
			"State": {
				Label: labelPrefix + " connection status",
				Unit:  mp.UnitInteger,
				Metrics: []mp.Metrics{
					// The state of the connection.1 indicates up and 0 indicates down.
					{Name: "ConnectionState", Label: "ConnectionState"},
				},
			},
		}
	}
}

// MetricKeyPrefix : interface for PluginWithPrefix
func (p AwsDxCon) MetricKeyPrefix() string {
	if p.Prefix == "" {
		p.Prefix = "Dx"
	}
	return p.Prefix
}

// FetchMetrics : fetch metrics
func (p AwsDxCon) FetchMetrics() (map[string]float64, error) {
	stat := make(map[string]float64)

	for _, met := range []metrics{
		{Name: "ConnectionState", Type: types.StatisticMinimum, FullSpecSupport: false},
		{Name: "ConnectionEncryptionState", Type: types.StatisticMinimum, FullSpecSupport: true},
		{Name: "ConnectionBpsEgress", Type: types.StatisticAverage, FullSpecSupport: true},
		{Name: "ConnectionBpsIngress", Type: types.StatisticAverage, FullSpecSupport: true},
		{Name: "ConnectionPpsEgress", Type: types.StatisticAverage, FullSpecSupport: true},
		{Name: "ConnectionPpsIngress", Type: types.StatisticAverage, FullSpecSupport: true},
		{Name: "ConnectionLightLevelTx", Type: types.StatisticAverage, FullSpecSupport: true},
		{Name: "ConnectionLightLevelRx", Type: types.StatisticAverage, FullSpecSupport: true},
		{Name: "ConnectionErrorCount", Type: types.StatisticSum, FullSpecSupport: true},
	} {
		if (met.FullSpecSupport == false) || (met.FullSpecSupport == true && p.FullSpecSupport == true) {
			v, err := p.getLastPoint(met)
			if err != nil {
				log.Printf("%v : %s", met, err)
			}
			stat[met.Name] = v
		}
	}
	return stat, nil
}

func (p AwsDxCon) getLastPoint(metric metrics) (float64, error) {
	now := time.Now()
	dimensions := []types.Dimension{
		{
			Name:  aws.String("ConnectionId"),
			Value: aws.String(p.DxConId),
		},
	}

	input := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		Dimensions: dimensions,
		StartTime:  aws.Time(now.Add(time.Duration(180) * time.Second * -1)), // 3 min (to fetch at least 1 data-point)
		EndTime:    aws.Time(now),
		Period:     aws.Int32(60),
		MetricName: aws.String(metric.Name),
		Statistics: []types.Statistic{metric.Type},
	}

	response, err := p.CloudWatch.GetMetricStatistics(context.Background(), input)
	if err != nil {
		return 0, err
	}

	datapoints := response.Datapoints
	if len(datapoints) == 0 {
		if metric.FullSpecSupport == true && p.FullSpecSupport == true {
			log.Printf("fetch no datapoints (%s may not be supported metric): %s", metric.Name, p.DxConId)
			return 0, nil
		} else {
			return 0, errors.New("fetch no datapoints (" + metric.Name + "): " + p.DxConId)
		}
	}

	// get least recently datapoint.
	// because a most recently datapoint is not stable.
	least := time.Now()
	var latestVal float64
	for _, dp := range datapoints {
		if dp.Timestamp.Before(least) {
			least = *dp.Timestamp
			switch metric.Type {
			case types.StatisticAverage:
				latestVal = *dp.Average
			case types.StatisticMaximum:
				latestVal = *dp.Maximum
			case types.StatisticMinimum:
				latestVal = *dp.Minimum
			case types.StatisticSum:
				latestVal = *dp.Sum
			}
		}
	}

	return latestVal, nil
}

func (p *AwsDxCon) prepare() error {
	var opts []func(*config.LoadOptions) error

	if p.RoleArn != "" {
		cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			return err
		}
		stsclient := sts.NewFromConfig(cfg)

		appCreds := stscreds.NewAssumeRoleProvider(stsclient, p.RoleArn)
		opts = append(opts, config.WithCredentialsProvider(appCreds))
	} else if p.AccessKeyID != "" && p.SecretKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(p.AccessKeyID, p.SecretKeyID, "")))
	}

	if p.Region != "" {
		opts = append(opts, config.WithRegion(p.Region))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return err
	}

	p.CloudWatch = cloudwatch.NewFromConfig(cfg)

	return nil
}

// Do : Do plugin
func Do() {
	optPrefix := flag.String("metric-key-prefix", "", "Metric Key Prefix")
	optAccessKeyID := flag.String("access-key-id", os.Getenv("AWS_ACCESS_KEY_ID"), "AWS Access Key ID")
	optSecretKeyID := flag.String("secret-key-id", os.Getenv("AWS_SECRET_ACCESS_KEY"), "AWS Secret Access Key ID")
	optRegion := flag.String("region", os.Getenv("AWS_DEFAULT_REGION"), "AWS Region")
	optRoleArn := flag.String("role-arn", "", "IAM Role ARN for assume role")
	optDxCon := flag.String("direct-connect-connection", "", "Resource ID of Direct Connect")
	optFullSpecSupport := flag.Bool("full-spec-support", true, "fetch all metrics")
	flag.Parse()

	var AwsDxCon AwsDxCon

	AwsDxCon.Prefix = *optPrefix
	AwsDxCon.AccessKeyID = *optAccessKeyID
	AwsDxCon.SecretKeyID = *optSecretKeyID
	AwsDxCon.Region = *optRegion
	AwsDxCon.RoleArn = *optRoleArn
	AwsDxCon.DxConId = *optDxCon
	AwsDxCon.FullSpecSupport = *optFullSpecSupport

	err := AwsDxCon.prepare()
	if err != nil {
		log.Fatalln(err)
	}

	helper := mp.NewMackerelPlugin(AwsDxCon)
	helper.Run()
}
