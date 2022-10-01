package main

import (
	"github.com/go-logr/zapr"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/controller/minecraftbackup"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/controller/minecraftserver"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(minecraftv1alpha1.AddToScheme(scheme))
}

func main() {
	// Flags and configuration management
	flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.Bool("leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()
	viper.BindPFlags(flag.CommandLine)

	// Logging
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	ctrl.SetLogger(zapr.NewLogger(log))
	defer log.Sync()

	// Manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     viper.GetString("metrics-bind-address"),
		Port:                   9443,
		HealthProbeBindAddress: viper.GetString("health-probe-bind-address"),
		LeaderElection:         viper.GetBool("leader-elect"),
		LeaderElectionID:       "95b821c2.jameslaverack.com",
	})
	if err != nil {
		log.With(zap.Error(err)).Fatal("Failed to start controller manager")
	}

	if err = (&minecraftserver.MinecraftServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		log.With(zap.Error(err), zap.String("controller", "MinecraftServer")).Fatal("Failed to setup controller")
	}

	if err = (&minecraftbackup.MinecraftBackupReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		log.With(zap.Error(err), zap.String("controller", "MinecraftBackup")).Fatal("Failed to setup controller")
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.With(zap.Error(err)).Fatal("Failed to setup health check endpoint")
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.With(zap.Error(err)).Fatal("Failed to setup ready check endpoint")
	}

	log.Info("Starting manager")
	if err := mgr.Start(logutil.IntoContext(ctrl.SetupSignalHandler(), log)); err != nil {
		log.With(zap.Error(err)).Fatal("Failed to run manager")
	}
}
