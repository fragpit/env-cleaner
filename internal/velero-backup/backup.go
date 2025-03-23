package velerobackup

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	clientset "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	backupTimeout = 10 * time.Minute
)

type VeleroBackup struct {
	VeleroClient    *clientset.Clientset
	VeleroNamespace string
}

func NewVeleroBackup(kubeConfig string,
	veleroNamespace string,
) (*VeleroBackup, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}

	clientSet, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &VeleroBackup{
		VeleroClient:    clientSet,
		VeleroNamespace: veleroNamespace,
	}, nil
}

func (v *VeleroBackup) Create(
	backupNamespace string,
	backupName string,
	ttl time.Duration,
) error {
	log.Infof("Creating Velero backup with name: %s", backupName)

	backupObj := velerov1api.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupName,
			Namespace: v.VeleroNamespace,
		},
		Spec: velerov1api.BackupSpec{
			IncludedNamespaces: []string{backupNamespace},
			TTL:                metav1.Duration{Duration: ttl},
		},
	}

	_, err := v.VeleroClient.VeleroV1().
		Backups(v.VeleroNamespace).
		Create(context.Background(), &backupObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if err := checkBackupStatus(v, backupName); err != nil {
		return err
	}

	log.Infof("Backup request %q submitted successfully.\n", backupName)

	return nil
}

func checkBackupStatus(v *VeleroBackup, backupName string) error {
	var err error
	backupStatus := &velerov1api.Backup{}
	timeout := time.After(backupTimeout)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("backup %q timeout", backupName)
		default:
			if backupStatus.Status.Phase == velerov1api.BackupPhaseCompleted {
				return nil
			}
			if backupStatus.Status.Phase == velerov1api.BackupPhaseFailedValidation ||
				backupStatus.Status.Phase == velerov1api.BackupPhasePartiallyFailed ||
				backupStatus.Status.Phase == velerov1api.BackupPhaseFailed {
				return fmt.Errorf("backup %q failed with status %q", backupName, backupStatus.Status.Phase)
			}

			time.Sleep(5 * time.Second)
			backupStatus, err = v.VeleroClient.VeleroV1().Backups(v.VeleroNamespace).
				Get(context.Background(), backupName, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}
	}
}
