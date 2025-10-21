package cmd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	client "argocd-pod-enrichment-webhook/pkg/kubernetesclient"

	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	tlsCert string
	tlsKey  string
	port    int
	codecs  = serializer.NewCodecFactory(runtime.NewScheme())
	logger  = log.New(os.Stdout, "http: ", log.LstdFlags)
)

var rootCmd = &cobra.Command{
	Use:   "mutating-webhook",
	Short: "Kubernetes mutating webhook example",
	Long: `Example showing how to implement a basic mutating webhook in Kubernetes.

Example:
$ mutating-webhook --tls-cert <tls_cert> --tls-key <tls_key> --port <port>`,
	Run: func(cmd *cobra.Command, args []string) {
		if tlsCert == "" || tlsKey == "" {
			fmt.Println("--tls-cert and --tls-key required")
			os.Exit(1)
		}
		runWebhookServer(tlsCert, tlsKey)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.Flags().StringVar(&tlsCert, "tls-cert", "/certs/tls.crt", "Certificate for TLS")
	rootCmd.Flags().StringVar(&tlsKey, "tls-key", "/certs/tls.key", "Private key file for TLS")
	rootCmd.Flags().IntVar(&port, "port", 8443, "Port to listen on for HTTPS traffic")
}

func admissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	// Validate that the incoming content type is correct.
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("expected application/json content-type")
	}

	// Get the body data, which will be the AdmissionReview
	// content for the request.
	var body []byte
	if r.Body != nil {
		requestData, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = requestData
	}

	// Decode the request body into
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		return nil, err
	}

	return admissionReviewRequest, nil
}

func mutatePod(w http.ResponseWriter, r *http.Request) {
	logger.Printf("received message on mutate")

	deserializer := codecs.UniversalDeserializer()

	// Parse the AdmissionReview from the http request.
	admissionReviewRequest, err := admissionReviewFromRequest(r, deserializer)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Printf(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}

	// Do server-side validation that we are only dealing with a pod resource. This
	// should also be part of the MutatingWebhookConfiguration in the cluster, but
	// we should verify here before continuing.
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		msg := fmt.Sprintf("did not receive pod, got %s", admissionReviewRequest.Request.Resource.Resource)
		logger.Printf(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}

	// Decode the pod from the AdmissionReview.
	rawRequest := admissionReviewRequest.Request.Object.Raw

	pod := unstructured.Unstructured{}

	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		msg := fmt.Sprintf("error converting raw pod to unstructured: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	client, err := client.NewInClusterKubernetesClient()
	if err != nil {
		msg := fmt.Sprintf("error creating in-cluster kubernetes client: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	owner, err := client.GetTopmostControllerOwner(&pod)

	if err != nil {
		msg := fmt.Sprintf("error getting topmost controller owner: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	logger.Printf("topmost owner of pod %s/%s is %s/%s", pod.GetNamespace(), pod.GetName(), owner.GetKind(), owner.GetName())

	// Create a response that will add a label to the pod if it does
	// not already have a label with the key of "hello". In this case
	// it does not matter what the value is, as long as the key exists.
	admissionResponse := &admissionv1.AdmissionResponse{}
	var patch string
	patchType := admissionv1.PatchTypeJSONPatch

	if _, ok := pod.GetLabels()["hello"]; !ok {
		patch = `[{"op":"add","path":"/metadata/labels/hello", "value": "world"}]`
	}

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
	}

	// Construct the response, which is just another AdmissionReview.
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID

	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		msg := fmt.Sprintf("error marshalling response json: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func runWebhookServer(certFile, keyFile string) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("Starting webhook server")
	http.HandleFunc("/mutate", mutatePod)
	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		ErrorLog: logger,
	}

	if err := server.ListenAndServeTLS("", ""); err != nil {
		panic(err)
	}
}
