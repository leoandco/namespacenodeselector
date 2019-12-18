package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/logger"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"net/http"
	"strings"
)

func parseNodeSelector(selector string) (map[string]string, error) {
	nodeSelector := make(map[string]string)
	for _, s := range strings.Split(selector, ",") {
		if !strings.Contains(s, "=") {
			return nil, errors.New("each node selector must be have a '=' character separating the label name and value")
		}
		split := strings.SplitN(s, "=", 2)
		//noinspection ALL, see prior check
		nodeSelector[split[0]] = split[1]
	}
	return nodeSelector, nil
}

func Mutate(w http.ResponseWriter, request *http.Request) {
	var admissionReview admissionv1.AdmissionReview

	reply := func() {
		resp, err := json.Marshal(admissionReview)
		if err != nil {
			logger.Errorln("Failed to serialize AdmissionResponse.\n%v", err)
			http.Error(w, fmt.Sprintf("Failed to serialize AdmissionResponse.\n%v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("content-type", "application/json")
		_, err = w.Write(resp)
		if err != nil {
			logger.Errorln("Failed to write response.\n%v", err)
			return
		}
	}

	buffer, err := ioutil.ReadAll(request.Body)
	if err != nil {
		logger.Errorln(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	body := string(buffer)

	admissionReview = admissionv1.AdmissionReview{Response: &admissionv1.AdmissionResponse{}}
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(body), 512).Decode(&admissionReview)
	if err != nil {
		logger.Warningf("Invalid Kubernetes admission.k8s.io/v1beta1 AdmissionReview manifest.\n%v\n", err)
		http.Error(w, fmt.Sprintf("Invalid Kubernetes admission.k8s.io/v1beta1 AdmissionReview manifest.\n%v", err), http.StatusBadRequest)
		return
	}
	admissionReview.Response.Allowed = true
	admissionReview.Response.UID = admissionReview.Request.UID

	var pod corev1.Pod
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(admissionReview.Request.Object.Raw), 512).Decode(&pod)
	if err != nil {
		logger.Warningf("Invalid Kubernetes corev1 Pod manifest.\n%v\n", err)
		admissionReview.Response.Allowed = false
		admissionReview.Response.Result = &metav1.Status{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Invalid Kubernetes corev1 Pod manifest.\n%v", err),
		}
		reply()
		return
	}

	namespace, err := client.CoreV1().Namespaces().Get(admissionReview.Request.Namespace, metav1.GetOptions{})
	if err != nil {
		logger.Warningf("Could not get Kubernetes namespace '%s'.\n%v\n", admissionReview.Request.Namespace, err)
		admissionReview.Response.Allowed = false
		admissionReview.Response.Result = &metav1.Status{
			Code:    http.StatusBadRequest,
			Message: fmt.Sprintf("Could not get Kubernetes namespace '%s'.\n%v\n", admissionReview.Request.Namespace, err),
		}
		reply()
		return
	}

	if namespaceNodeSelectorYaml, ok := namespace.Annotations[NodeSelectorAnnotation]; ok && pod.Spec.NodeSelector == nil {
		nodeSelector, err := parseNodeSelector(namespaceNodeSelectorYaml)
		if err != nil {
			logger.Warningf("'%s' annotation on the '%s' namespace is not a valid corev1.NodeSelector object.\n%v\n", NodeSelectorAnnotation, err)
			admissionReview.Response.Allowed = false
			admissionReview.Response.Result = &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("'nodeselector' annotation on the '%s' namespace is not a valid corev1.NodeSelector object.\n%v\n", err),
			}
			reply()
			return
		}

		var patch []jsonPatch
		patch = append(patch, jsonPatch{
			Op:    "add",
			Path:  "/spec/nodeSelector",
			Value: nodeSelector,
		})
		for key, value := range nodeSelector {
			patch = append(patch, jsonPatch{
				Op:   "add",
				Path: "/spec/tolerations/-",
				Value: corev1.Toleration{
					Key:      key,
					Operator: corev1.TolerationOpEqual,
					Value:    value,
					Effect:   "NoSchedule",
				},
			})
		}

		patchBytes, err := json.Marshal(patch)
		if err != nil {
			logger.Errorf("Unable to serialize JSON patch.\n%v\n", err)
			admissionReview.Response.Allowed = false
			admissionReview.Response.Result = &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Unable to serialize JSON patch.\n%v\n", err),
			}
			reply()
			return
		}

		println(string(patchBytes))

		admissionReview.Response.Allowed = true
		admissionReview.Response.PatchType = func() *admissionv1.PatchType {
			ptr := admissionv1.PatchTypeJSONPatch
			return &ptr
		}()
		admissionReview.Response.Patch = patchBytes
		reply()
		return
	}

	reply()
}
