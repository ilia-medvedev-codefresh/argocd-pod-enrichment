package webhook

import (
	"crypto/tls"
	"encoding/json"
	"strings"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	client "argocd-pod-enrichment/pkg/kubernetesclient"
	argocdtracking "argocd-pod-enrichment/pkg/argocdresourcetracking"

	"github.com/spf13/cobra"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	consts "argocd-pod-enrichment/pkg/consts/webhook"
)

var (
	tlsCert string
	tlsKey  string
	port    int
	codecs  = serializer.NewCodecFactory(runtime.NewScheme())
	logger  = log.New(os.Stdout, "http: ", log.LstdFlags)
)

var WebhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Kubernetes mutating webhook",
	Long: `Run the mutating webhook server for ArgoCD pod enrichment.

Example:
$ argocd-pod-enrichment webhook --tls-cert <tls_cert> --tls-key <tls_key> --port <port>`,
	Run: func(cmd *cobra.Command, args []string) {
		if tlsCert == "" || tlsKey == "" {
			fmt.Println("--tls-cert and --tls-key required")
			os.Exit(1)
		}
		runWebhookServer(tlsCert, tlsKey)
	},
}

func init() {
	WebhookCmd.Flags().StringVar(&tlsCert, "tls-cert", "/certs/tls.crt", "Certificate for TLS")
	WebhookCmd.Flags().StringVar(&tlsKey, "tls-key", "/certs/tls.key", "Private key file for TLS")
	WebhookCmd.Flags().IntVar(&port, "port", 8443, "Port to listen on for HTTPS traffic")
}

func admissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("expected application/json content-type")
	}
	var body []byte
	if r.Body != nil {
		requestData, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = requestData
	}
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := codecs.UniversalDeserializer().Decode(body, nil, admissionReviewRequest); err != nil {
		return nil, err
	}
	return admissionReviewRequest, nil
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

func mutatePod(w http.ResponseWriter, r *http.Request) {
	logger.Print("received message on mutate")
	deserializer := codecs.UniversalDeserializer()
	admissionReviewRequest, err := admissionReviewFromRequest(r, deserializer)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Print(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		msg := fmt.Sprintf("did not receive pod, got %s", admissionReviewRequest.Request.Resource.Resource)
		logger.Print(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}
	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := unstructured.Unstructured{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		msg := fmt.Sprintf("error converting raw pod to unstructured: %v", err)
		logger.Print(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}
	client, err := client.NewInClusterKubernetesClient()
	if err != nil {
		msg := fmt.Sprintf("error creating in-cluster kubernetes client: %v", err)
		logger.Print(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}
	owner, err := client.GetTopmostControllerOwner(&pod)
	if err != nil {
		msg := fmt.Sprintf("error getting topmost controller owner: %v", err)
		logger.Print(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	argocdtracking := argocdtracking.ExtractArgoCDTrackingInfo(*owner)
	logger.Printf("Extracted ArgoCD tracking info: %+v", argocdtracking)

	if argocdtracking != nil {

		admissionReviewResponse := constructResponse(argocdtracking)
		admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
		admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID

		resp, err := json.Marshal(admissionReviewResponse)
		if err != nil {
			msg := fmt.Sprintf("error marshalling response json: %v", err)
			logger.Print(msg)
			w.WriteHeader(500)
			w.Write([]byte(msg))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}
}

func constructResponse(argocdtracking *argocdtracking.ArgoCDTrackingInfo) *admissionv1.AdmissionReview {
	admissionResponse := &admissionv1.AdmissionResponse{}
	var patch string
	patchType := admissionv1.PatchTypeJSONPatch

	appLabelPath := "/metadata/labels/" + strings.ReplaceAll(consts.ApplicationLabelKey, "/", "~1")
	patchOperations := []string{`{"op":"add","path":"` + appLabelPath + `", "value": "` + argocdtracking.ApplicationName + `"}`}

	if argocdtracking.ApplicationNamespace != "" {
		nsLabelPath := "/metadata/labels/" + strings.ReplaceAll(consts.ApplicationNamespaceLabelKey, "/", "~1")
		patchOperations = append(patchOperations, `{"op":"add","path":"` + nsLabelPath + `", "value": "` + argocdtracking.ApplicationNamespace + `"}`)
	}

	if argocdtracking.InstallationID != "" {
		idLabelPath := "/metadata/labels/" + strings.ReplaceAll(consts.InstallationIDLabelKey, "/", "~1")
		patchOperations = append(patchOperations, `{"op":"add","path":"` + idLabelPath + `", "value": "` + argocdtracking.InstallationID + `"}`)
	}

	patch = "[" +  strings.Join(patchOperations, ",") + "]"

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
	}
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse

	return &admissionReviewResponse
}
