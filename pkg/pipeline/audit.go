package pipeline

import (
	"context"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jbcom/secrets-sync/pkg/audit"
	log "github.com/sirupsen/logrus"
)

// buildAuditor constructs an audit.Logger from the audit config, combining
// file, S3, and CloudWatch sinks as configured. It returns nil when no
// destination is set (auditing disabled).
func (p *Pipeline) buildAuditor(ctx context.Context, cfg AuditConfig) (*audit.Logger, error) {
	var sinks []audit.Sink

	if cfg.File != "" {
		fs, err := audit.NewFileSink(cfg.File)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, fs)
	}

	if cfg.S3Bucket != "" || cfg.CloudWatchGroup != "" {
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.config.AWS.Region))
		if err != nil {
			return nil, err
		}
		if cfg.S3Bucket != "" {
			sinks = append(sinks, audit.NewS3Sink(s3.NewFromConfig(awsCfg), cfg.S3Bucket, cfg.S3Prefix))
		}
		if cfg.CloudWatchGroup != "" {
			sinks = append(sinks, audit.NewCloudWatchSink(cloudwatchlogs.NewFromConfig(awsCfg), cfg.CloudWatchGroup, cfg.CloudWatchStream))
		}
	}

	if len(sinks) == 0 {
		return nil, nil
	}
	return audit.NewLogger(audit.NewMultiSink(sinks...)), nil
}

// audit logs an audit record if an auditor is configured. Failures to write an
// audit entry are logged but never block the sync (the operation already
// happened); audit-sink reliability is an operational concern.
func (p *Pipeline) audit(ctx context.Context, r audit.Record) {
	if p.auditor == nil {
		return
	}
	if err := p.auditor.Log(ctx, r); err != nil {
		log.WithError(err).Warn("Failed to write audit entry")
	}
}
