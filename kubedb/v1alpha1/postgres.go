package v1alpha1

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/jsonpatch"
	"github.com/appscode/kutil"
	"github.com/golang/glog"
	aci "github.com/k8sdb/apimachinery/api"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

func EnsurePostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(alert *aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	return CreateOrPatchPostgres(c, meta, transform)
}

func CreateOrPatchPostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(alert *aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	cur, err := c.Postgreses(meta.Namespace).Get(meta.Name)
	if kerr.IsNotFound(err) {
		return c.Postgreses(meta.Namespace).Create(transform(&aci.Postgres{ObjectMeta: meta}))
	} else if err != nil {
		return nil, err
	}
	return PatchPostgres(c, cur, transform)
}

func PatchPostgres(c tcs.ExtensionInterface, cur *aci.Postgres, transform func(*aci.Postgres) *aci.Postgres) (*aci.Postgres, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(transform(cur))
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(curJson, modJson)
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 {
		return cur, nil
	}
	pb, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return nil, err
	}
	glog.V(5).Infof("Patching Postgres %s@%s with %s.", cur.Name, cur.Namespace, string(pb))
	result, err := c.Postgreses(cur.Namespace).Patch(cur.Name, types.JSONPatchType, pb)
	return result, err
}

func TryPatchPostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(*aci.Postgres) *aci.Postgres) (result *aci.Postgres, err error) {
	attempt := 0
	err = wait.Poll(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.Postgreses(meta.Namespace).Get(meta.Name)
		if kerr.IsNotFound(e2) {
			return true, e2
		} else if e2 == nil {
			result, e2 = PatchPostgres(c, cur, transform)
			return e2 == nil, e2
		}
		glog.Errorf("Attempt %d failed to patch Postgres %s@%s due to %v.", attempt, cur.Name, cur.Namespace, e2)
		return false, e2
	})

	if err != nil {
		err = fmt.Errorf("Failed to patch Postgres %s@%s after %d attempts due to %v", meta.Name, meta.Namespace, attempt, err)
	}
	return
}

func TryUpdatePostgres(c tcs.ExtensionInterface, meta metav1.ObjectMeta, transform func(*aci.Postgres) *aci.Postgres) (result *aci.Postgres, err error) {
	attempt := 0
	err = wait.Poll(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.Postgreses(meta.Namespace).Get(meta.Name)
		if kerr.IsNotFound(e2) {
			return true, e2
		} else if e2 == nil {
			result, e2 = c.Postgreses(cur.Namespace).Update(transform(cur))
			return e2 == nil, e2
		}
		glog.Errorf("Attempt %d failed to update Postgres %s@%s due to %v.", attempt, cur.Name, cur.Namespace, e2)
		return false, e2
	})

	if err != nil {
		err = fmt.Errorf("Failed to update Postgres %s@%s after %d attempts due to %v", meta.Name, meta.Namespace, attempt, err)
	}
	return
}
