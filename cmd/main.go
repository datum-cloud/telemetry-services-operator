/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	"sigs.k8s.io/multicluster-runtime/pkg/multicluster"
	mckind "sigs.k8s.io/multicluster-runtime/providers/kind"
	mcsingle "sigs.k8s.io/multicluster-runtime/providers/single"

	telemetryv1alpha1 "go.datum.net/telemetry-services-operator/api/v1alpha1"
	"go.datum.net/telemetry-services-operator/internal/controller"
	"go.datum.net/telemetry-services-operator/internal/providers"
	mcdatum "go.datum.net/telemetry-services-operator/internal/providers/datum"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var vectorConfigLabelKey, vectorConfigLabelValue string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var clusterDiscoveryMode string
	var vectorConfigurationNamespace string
	var upstreamClusterKubeconfig string
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(
		&vectorConfigLabelKey,
		"vector-config-label-key",
		"telemetry.datumapis.com/vector-export-policy-config",
		"The key of the label that will be added to the vector config secret.",
	)
	flag.StringVar(&vectorConfigLabelValue,
		"vector-config-label-value",
		"true",
		"The value of the label that will be added to the vector config secret.",
	)
	flag.StringVar(&clusterDiscoveryMode, "cluster-discovery-mode", "single",
		"Method to discover clusters. Allowed values are: "+strings.Join(providers.AllowedProviders, ","))
	flag.StringVar(&vectorConfigurationNamespace, "vector-config-namespace", "default",
		"The namespace in the downstream cluster to create the vector config secret in.")
	flag.StringVar(&upstreamClusterKubeconfig, "upstream-cluster-kubeconfig", "", "The path to the kubeconfig file to use for connecting to the upstream cluster.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	upstreamClusterConfig, err := clientcmd.BuildConfigFromFlags("", upstreamClusterKubeconfig)
	if err != nil {
		setupLog.Error(err, "unable to load control plane kubeconfig")
		os.Exit(1)
	}

	// Retrieve the configuration for the cluster that the operator is deployed
	// in. This is the cluster that will have the vector config secret created
	// for telemetry services.
	downstreamClusterConfig := ctrl.GetConfigOrDie()

	downstreamCluster, err := cluster.New(downstreamClusterConfig, func(o *cluster.Options) {
		o.Scheme = scheme
	})
	if err != nil {
		setupLog.Error(err, "failed to construct downstream luster")
		os.Exit(1)
	}

	var localManager manager.Manager

	var provider interface {
		multicluster.Provider
		// TODO(jreese) see if Run should be defined in the Provider interface
		Run(context.Context, mcmanager.Manager) error
	}
	var singleCluster cluster.Cluster

	switch clusterDiscoveryMode {
	case providers.ProviderSingle:
		singleCluster, err = cluster.New(downstreamClusterConfig, func(o *cluster.Options) {
			o.Scheme = scheme
		})
		if err != nil {
			setupLog.Error(err, "failed creating cluster")
			os.Exit(1)
		}
		provider = mcsingle.New("single", singleCluster)

	case providers.ProviderDatum:
		localManager, err = manager.New(downstreamClusterConfig, manager.Options{
			Client: client.Options{
				Cache: &client.CacheOptions{
					Unstructured: true,
				},
			},
		})
		if err != nil {
			setupLog.Error(err, "unable to set up overall controller manager")
			os.Exit(1)
		}

		provider, err = mcdatum.New(localManager, upstreamClusterConfig, mcdatum.Options{
			ClusterOptions: []cluster.Option{
				func(o *cluster.Options) {
					o.Scheme = scheme
				},
			},
		})
		if err != nil {
			setupLog.Error(err, "unable to create datum project provider")
			os.Exit(1)
		}

	case providers.ProviderKind:
		provider = mckind.New()

	default:
		setupLog.Error(fmt.Errorf(
			"unsupported cluster discovery mode. Got %q, expected one of %s",
			clusterDiscoveryMode,
			strings.Join(providers.AllowedProviders, ","),
		), "")
		os.Exit(1)
	}

	mgr, err := mcmanager.New(upstreamClusterConfig, provider, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "telemetry.datumapis.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.ExportPolicyReconciler{
		DownstreamClient:                downstreamCluster.GetClient(),
		DownstreamVectorConfigNamespace: vectorConfigurationNamespace,
		MetricsService: controller.MetricsService{
			Endpoint: os.Getenv("TELEMETRY_SERVICE_METRICS_ENDPOINT"),
			Username: os.Getenv("TELEMETRY_SERVICE_METRICS_USERNAME"),
			Password: os.Getenv("TELEMETRY_SERVICE_METRICS_PASSWORD"),
		},
		VectorConfigLabelKey:   vectorConfigLabelKey,
		VectorConfigLabelValue: vectorConfigLabelValue,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ExportPolicy")
		os.Exit(1)
	}
	// nolint:goconst
	if os.Getenv("ENABLE_WEBHOOKS") != "false" {
		// TODO: Re-enable webhooks once the webhook server is properly configured
		//       to support multicluster.
		//
		// if err = webhooktelemetryv1alpha1.SetupExportPolicyWebhookWithManager(mgr); err != nil {
		// 	setupLog.Error(err, "unable to create webhook", "webhook", "ExportPolicy")
		// 	os.Exit(1)
		// }
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()

	if clusterDiscoveryMode == providers.ProviderSingle {
		setupLog.Info("engaging cluster for single cluster provider")
		// Pending feedback on https://github.com/multicluster-runtime/multicluster-runtime/pull/17#issue-2911191237
		// to determine if the provider's Run function should be calling Engage
		if err := mgr.Engage(ctx, "single", singleCluster); err != nil {
			setupLog.Error(err, "failed engaging cluster")
			os.Exit(1)
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	if localManager != nil {
		setupLog.Info("starting local manager")
		g.Go(func() error {
			return ignoreCanceled(localManager.Start(ctx))
		})
	}

	setupLog.Info("starting cluster discovery provider")
	g.Go(func() error {
		return ignoreCanceled(provider.Run(ctx, mgr))
	})

	g.Go(func() error {
		return ignoreCanceled(downstreamCluster.Start(ctx))
	})

	if singleCluster != nil {
		setupLog.Info("starting cluster for single cluster provider")
		g.Go(func() error {
			return ignoreCanceled(singleCluster.Start(ctx))
		})
	}

	setupLog.Info("starting multicluster manager")
	g.Go(func() error {
		return ignoreCanceled(mgr.Start(ctx))
	})

	if err := g.Wait(); err != nil {
		setupLog.Error(err, "unable to start")
		os.Exit(1)
	}
}

func ignoreCanceled(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
