package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	secretskubernetes "github.com/jbcom/secrets-sync/pkg/kubernetes"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
	clientkubernetes "k8s.io/client-go/kubernetes"
)

func main() {
	var kubeconfig string
	var namespace string
	var resync time.Duration
	var image string
	var logLevel string
	var logFormat string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig; defaults to in-cluster, KUBECONFIG, then ~/.kube/config")
	flag.StringVar(&namespace, "namespace", "", "namespace to watch; empty watches all namespaces")
	flag.DurationVar(&resync, "resync", time.Minute, "controller resync period")
	flag.StringVar(&image, "image", secretskubernetes.DefaultImage, "default secrets-sync image for managed CronJobs")
	flag.StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	flag.StringVar(&logFormat, "log-format", "text", "log format: text or json")
	flag.Parse()

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		level = log.InfoLevel
	}
	log.SetLevel(level)
	if logFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	restConfig, err := secretskubernetes.BuildRESTConfig(kubeconfig)
	if err != nil {
		exit(fmt.Errorf("build Kubernetes config: %w", err))
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		exit(fmt.Errorf("create dynamic client: %w", err))
	}
	kubeClient, err := clientkubernetes.NewForConfig(restConfig)
	if err != nil {
		exit(fmt.Errorf("create Kubernetes client: %w", err))
	}

	controller := secretskubernetes.NewController(dynamicClient, kubeClient, secretskubernetes.Config{
		Namespace:    namespace,
		ResyncPeriod: resync,
		DefaultImage: image,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.WithFields(log.Fields{
		"namespace": namespace,
		"resync":    resync.String(),
		"image":     image,
	}).Info("Starting secrets-sync Kubernetes controller")

	if err := controller.Run(ctx); err != nil && ctx.Err() == nil {
		exit(err)
	}
}

func exit(err error) {
	log.WithError(err).Error("secrets-sync controller failed")
	os.Exit(1)
}
