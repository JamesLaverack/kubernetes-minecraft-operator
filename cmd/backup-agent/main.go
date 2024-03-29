package main

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	rcon "github.com/katnegermis/pocketmine-rcon"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
)

func main() {
	log, err := zap.NewProduction()
	defer log.Sync()

	serverObjectName := os.Getenv("SERVER_OBJECT_NAME")
	serverObjectNamespace := os.Getenv("SERVER_OBJECT_NAME")
	backupName := os.Getenv("BACKUP_NAME")
	rconAddress := os.Getenv("RCON_ADDRESS")
	backupSourceDIR := os.Getenv("BACKUP_SOURCE_DIR")
	backupDestPath := os.Getenv("BACKUP_DEST_PATH")

	log.With(zap.String("server-object-name", serverObjectName),
		zap.String("server-object-namespace", serverObjectNamespace),
		zap.String("backup-name", backupName),
		zap.String("rcon-address", rconAddress),
		zap.String("backup-source-dir", backupSourceDIR),
		zap.String("backup-dest-path", backupDestPath)).Info("Starting backup")

	config, err := rest.InClusterConfig()
	if err != nil {
		log.With(zap.Error(err)).Panic("Failed to get in-cluster config")
	}
	log.Info("Acquired in-cluster Kube connection")

	v1alpha1.AddToScheme(scheme.Scheme)

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: "minecraft.jameslaverack.com", Version: "v1alpha1"}
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	restClient, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		log.With(zap.Error(err)).Panic("Failed to create REST Client")
	}

	ctx := context.Background()

	log.Info("Acquiring lease")
	err = acquireLease(ctx, restClient, serverObjectName, serverObjectNamespace, backupName)
	if err != nil {
		log.With(zap.Error(err)).Panic("Failed to acquire lease")
	}
	log.Info("Lease acquired")

	// TODO make sure to time out after the lease expires!

	// TODO Use a real password
	conn, err := rcon.NewConnection(rconAddress, "password")
	if err != nil {
		log.With(zap.Error(err), zap.String("rcon-address", rconAddress)).Panic("Failed to connect to rcon")
	}
	_, err = conn.SendCommand("save-off")
	if err != nil {
		log.With(zap.Error(err), zap.String("rcon-address", rconAddress)).Panic("Failed to send save-off")
	}
	_, err = conn.SendCommand("save-all")
	if err != nil {
		log.With(zap.Error(err), zap.String("rcon-address", rconAddress)).Panic("Failed to send save-all")
	}

	file, err := os.Create(filepath.Join(backupDestPath, backupName+".zip"))
	if err != nil {
		log.With(zap.Error(err), zap.String("backup-dest-path", backupDestPath)).Panic("Failed to create backup destination")
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	walker := func(path string, info os.FileInfo, err error) error {
		log.With(zap.String("path", path)).Debug("Crawling path")
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		f, err := w.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}

		return nil
	}
	err = filepath.Walk(backupSourceDIR, walker)
	if err != nil {
		log.With(zap.Error(err), zap.String("backup-dest-path", backupDestPath)).Panic("Failed to create backup")
	}

	_, err = conn.SendCommand("save-on")
	if err != nil {
		log.With(zap.Error(err), zap.String("rcon-address", rconAddress)).Panic("Failed to send save-on")
	}

	// Done!
}

func acquireLease(ctx context.Context, client *rest.RESTClient, serverObjectName, serverObjectNamespace, name string) error {
	for {
		result := client.Get().Resource("minecraftservers").Name(serverObjectName).Namespace(serverObjectNamespace).Do(ctx)
		if result.Error() != nil {
			return result.Error()
		}

		var server v1alpha1.MinecraftServer
		err := result.Into(&server)
		if err != nil {
			return err
		}

		// Is there a lease already?
		const ownerAnnotation = "minecraft/backup-lease-owner"
		const expiryAnnotation = "minecraft/backup-lease-expiry"
		currentOwner, ok := server.Annotations[ownerAnnotation]
		currentLeaseExpiry, err := time.Parse(time.RFC3339, server.Annotations[expiryAnnotation])

		if ok && err != nil && currentLeaseExpiry.After(time.Now()) {
			// There is already a valid lease
			// Is it us?
			if currentOwner == name {
				// Hey that's us!
				return nil
			}
			// Oh, not us, wait until it expires
			time.Sleep(currentLeaseExpiry.Sub(time.Now()))
		}

		// Okay, the lease is expired or invalid, try to get it.
		server.Annotations[ownerAnnotation] = name
		server.Annotations[expiryAnnotation] = time.Now().Add(time.Minute * 10).Format(time.RFC3339)

		result = client.Put().Resource("minecraftservers").Name(serverObjectName).Namespace(serverObjectNamespace).Body(&server).Do(ctx)
		if result.Error() == nil {
			// Done!
			return nil
		}
		// Special case, just retry!
	}
}
