package main

import (
	"fmt"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
)

type jsonPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

const AppName = "nsns"
const NodeSelectorAnnotation = "node-selector"

var client *kubernetes.Clientset
var cmd cobra.Command

func init() {
	logger.Init(AppName, true, false, ioutil.Discard)

	cmd = cobra.Command{
		Use: AppName,
	}
	viper.SetEnvPrefix(AppName)
	viper.AutomaticEnv()
}

func main() {
	cmd.Flags().String("addr", "0.0.0.0", "ip addr to bind the webhook server to")
	cmd.Flags().Int("port", 80, "port to bind the webhook server to")
	cmd.Flags().String("cert-path", "cert.crt", "path to the tls certificate")
	cmd.Flags().String("key-path", "key.", "path to the tls private key")
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		logger.Fatal(err)
	}

	addr := viper.GetString("addr")
	port := viper.GetInt("port")
	certPath := viper.GetString("certPath")
	keyPath := viper.GetString("keyPath")

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		logger.Fatalf("The TLS certificate does not exist: %s\n", err)
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		logger.Fatalf("The TLS private key does not exist: %s\n", err)
	}

	config, err := clientcmd.BuildConfigFromFlags("", "/home/leoxiong/.kube/config")
	if err != nil {
		log.Fatal(err)
	}
	client = kubernetes.NewForConfigOrDie(config)

	http.HandleFunc("/mutate", Mutate)
	log.Fatal(http.ListenAndServeTLS(fmt.Sprintf("%s:%d", addr, port), certPath, keyPath, nil))
}
