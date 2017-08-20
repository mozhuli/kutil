package kutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	. "github.com/appscode/go/types"
	"github.com/cenkalti/backoff"
	"github.com/golang/glog"
	"github.com/mattbaird/jsonpatch"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

func PatchRC(c clientset.Interface, cur *apiv1.ReplicationController, transform func(*apiv1.ReplicationController)) (*apiv1.ReplicationController, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	transform(cur)
	modJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(curJson, modJson)
	if err != nil {
		return nil, err
	}
	pb, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return nil, err
	}
	return c.CoreV1().ReplicationControllers(cur.Namespace).Patch(cur.Name, types.JSONPatchType, pb)
}

func TryPatchRC(c clientset.Interface, meta metav1.ObjectMeta, transform func(*apiv1.ReplicationController)) (*apiv1.ReplicationController, error) {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := c.CoreV1().ReplicationControllers(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			return PatchRC(c, cur, transform)
		}
		glog.Errorf("Attempt %d failed to patch ReplicationController %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return nil, fmt.Errorf("Failed to patch ReplicationController %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

func UpdateRC(c clientset.Interface, meta metav1.ObjectMeta, transform func(*apiv1.ReplicationController)) (*apiv1.ReplicationController, error) {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := c.CoreV1().ReplicationControllers(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return cur, err
		} else if err == nil {
			transform(cur)
			return c.CoreV1().ReplicationControllers(cur.Namespace).Update(cur)
		}
		glog.Errorf("Attempt %d failed to update ReplicationController %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return nil, fmt.Errorf("Failed to update ReplicationController %s@%s after %d attempts.", meta.Name, meta.Namespace, attempt)
}

func WaitUntilRCReady(c clientset.Interface, meta metav1.ObjectMeta) error {
	return backoff.Retry(func() error {
		if obj, err := c.CoreV1().ReplicationControllers(meta.Namespace).Get(meta.Name, metav1.GetOptions{}); err == nil {
			if Int32(obj.Spec.Replicas) == obj.Status.ReadyReplicas {
				return nil
			}
		}
		return errors.New("check again")
	}, backoff.NewConstantBackOff(2*time.Second))
}
