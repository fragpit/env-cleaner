package helm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/xhit/go-str2duration/v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/model"
	velerobackup "github.com/fragpit/env-cleaner/internal/velero-backup"
	"github.com/fragpit/env-cleaner/pkg/utils"
)

const (
	helmDeleteTimeout = 300 * time.Second
	connectorType     = "helm"
)

type Connector struct {
	HelmClient  *cli.EnvSettings
	KubeClient  *kubernetes.Clientset
	Cfg         Config
	Notificator model.Notificator
}

type Config struct {
	EnvCfg  config.Helm
	ConnCfg config.K8s
}

var _ model.Connector = (*Connector)(nil)

func New(cfg *Config, nt model.Notificator) (*Connector, error) {
	if cfg.ConnCfg.Kubeconfig == "" {
		return nil, errors.New("kubeconfig is empty")
	}

	helmClient := cli.New()
	helmClient.KubeConfig = cfg.ConnCfg.Kubeconfig

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", cfg.ConnCfg.Kubeconfig)
	if err != nil {
		return nil, errors.New("error building kubeconfig")
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.New("error creating kubernetes client")
	}

	return &Connector{
		HelmClient:  helmClient,
		KubeClient:  kubeClient,
		Cfg:         *cfg,
		Notificator: nt,
	}, nil
}

func (h *Connector) GetEnvironments(
	_ context.Context,
) ([]model.Environment, error) {
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(h.HelmClient.RESTClientGetter(), "", "", log.Printf); err != nil {
		return nil, fmt.Errorf("error getting releases: %w", err)
	}

	client := action.NewList(actionConfig)
	client.Deployed = true

	var filter string
	for i, rule := range h.Cfg.EnvCfg.WhitelistReleasesRegex {
		exp := "(" + rule + ")"
		if i != len(h.Cfg.EnvCfg.WhitelistReleasesRegex)-1 {
			exp += "|"
		}
		filter += exp
	}
	client.Filter = filter

	results, err := client.Run()
	if err != nil {
		return nil, fmt.Errorf("error getting releases: %w", err)
	}

	if len(results) == 0 {
		log.Infof("No helm releases found for specified filter: %s", filter)
		return nil, nil
	}

	envs := make([]model.Environment, 0, len(results))
	for _, rel := range results {
		if slices.Contains(h.Cfg.EnvCfg.BlacklistNamespaces, rel.Namespace) {
			log.Warnf(
				"Skipped helm release %s (%s): blacklisted",
				rel.Name,
				rel.Namespace,
			)
			continue
		}

		owner := getChartValue(rel, "ec_owner")
		ttl := getChartValue(rel, "ec_ttl")
		if owner == "" || ttl == "" {
			log.Warnf(
				"Skipped helm release %s (%s): owner or ttl is empty",
				rel.Name,
				rel.Namespace,
			)
			envTmp := &model.Environment{
				Name:      rel.Name,
				Namespace: rel.Namespace,
				Type:      connectorType,
			}
			if err := h.Notificator.SendOrphanMessage(envTmp); err != nil {
				return nil, fmt.Errorf("error processing: %w", err)
			}
			continue
		}

		deleteAt, deleteAtSec, err := utils.SetDeleteAt(ttl)
		if err != nil {
			log.Warnf(
				"Skipped helm release %s (%s): error setting deleteAt: %v",
				rel.Name,
				rel.Namespace,
				err,
			)
			continue
		}

		envID := strconv.Itoa(int(rel.Info.FirstDeployed.Unix()))
		env := model.Environment{
			EnvID:       envID,
			Type:        connectorType,
			Name:        rel.Name,
			Namespace:   rel.Namespace,
			Owner:       owner,
			DeleteAt:    deleteAt,
			DeleteAtSec: deleteAtSec,
		}
		envs = append(envs, env)
	}

	return envs, nil
}

func (h *Connector) GetEnvironmentID(
	_ context.Context,
	env *model.Environment,
) (string, error) {
	h.HelmClient.SetNamespace(env.Namespace)
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(h.HelmClient.RESTClientGetter(), env.Namespace, "", log.Printf); err != nil {
		return "", fmt.Errorf("error getting release: %w", err)
	}

	client := action.NewStatus(actionConfig)
	client.ShowResources = false
	client.ShowResourcesTable = false

	rel, err := client.Run(env.Name)
	if err != nil {
		return "", fmt.Errorf("error getting release: %w", err)
	}

	envID := strconv.Itoa(int(rel.Info.FirstDeployed.Unix()))

	return envID, nil
}

func (h *Connector) CheckEnvironment(
	ctx context.Context,
	env *model.Environment,
) error {
	envID, err := h.GetEnvironmentID(ctx, env)
	if err != nil {
		return fmt.Errorf("error checking environment: %w", err)
	}

	if envID != env.EnvID {
		return fmt.Errorf("error getting release: environment ID changed")
	}

	return nil
}

func (h *Connector) DeleteEnvironment(
	ctx context.Context,
	env *model.Environment,
) error {
	if h.Cfg.EnvCfg.VeleroBackup.Enabled {
		if err := h.scaleAndBackup(ctx, env); err != nil {
			return err
		}
	}

	h.HelmClient.SetNamespace(env.Namespace)
	actionConfig := new(action.Configuration)

	if err := actionConfig.Init(h.HelmClient.RESTClientGetter(), env.Namespace, "", log.Debugf); err != nil {
		return fmt.Errorf("error deleting release: %w", err)
	}

	client := action.NewUninstall(actionConfig)
	client.Timeout = helmDeleteTimeout
	client.DeletionPropagation = "background"
	client.Wait = true

	if _, err := client.Run(env.Name); err != nil {
		return fmt.Errorf("error deleting release: %w", err)
	}

	if h.Cfg.EnvCfg.DeleteReleaseNamespace {
		nsManifest := &apiv1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: env.Namespace,
				Labels: map[string]string{
					"name": env.Namespace,
				},
			},
		}

		buf, err := yaml.Marshal(nsManifest)
		if err != nil {
			return fmt.Errorf("error deleting release: %w", err)
		}

		resourseList, err := actionConfig.KubeClient.Build(
			bytes.NewBuffer(buf),
			false,
		)
		if err != nil {
			return fmt.Errorf("error deleting release: %w", err)
		}

		if _, err := actionConfig.KubeClient.Delete(resourseList); err != nil {
			var rerr string
			for _, e := range err {
				rerr += e.Error()
			}
			return fmt.Errorf("error deleting release: %w", errors.New(rerr))
		}
	}

	return nil
}

func (h *Connector) GetConnectorType() string {
	return connectorType
}

func getChartValue(rel *release.Release, key string) string {
	if rel.Config[key] == nil {
		return ""
	}

	return rel.Config[key].(string)
}

func scaleDeployment(
	ctx context.Context,
	client v1.DeploymentInterface,
	deploymentName string,
	replicas int32,
) error {
	s, err := client.GetScale(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	sc := *s
	sc.Spec.Replicas = replicas

	if _, err = client.UpdateScale(ctx, deploymentName, &sc, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func scaleStatefulSet(
	ctx context.Context,
	client v1.StatefulSetInterface,
	statefulsetName string,
	replicas int32,
) error {
	s, err := client.GetScale(ctx, statefulsetName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	sc := *s
	sc.Spec.Replicas = replicas

	if _, err := client.UpdateScale(ctx, statefulsetName, &sc, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (h *Connector) scaleAndBackup(
	ctx context.Context,
	env *model.Environment,
) error {
	deploymentsClient := h.KubeClient.AppsV1().Deployments(env.Namespace)
	statefulSetClient := h.KubeClient.AppsV1().StatefulSets(env.Namespace)

	deployments, err := deploymentsClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	sts, err := statefulSetClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	for i := range deployments.Items {
		d := &deployments.Items[i]
		if err := scaleDeployment(ctx, deploymentsClient, d.Name, 0); err != nil {
			return err
		}
	}

	for i := range sts.Items {
		s := &sts.Items[i]
		if err := scaleStatefulSet(ctx, statefulSetClient, s.Name, 0); err != nil {
			return err
		}
	}

	veleroBackupTTL, err := str2duration.ParseDuration(
		h.Cfg.EnvCfg.VeleroBackup.TTL,
	)
	if err != nil {
		return err
	}
	veleroBackupName := fmt.Sprintf("ec-backup-%s-%s", env.Namespace, env.EnvID)
	backup, err := velerobackup.NewVeleroBackup(
		h.Cfg.ConnCfg.Kubeconfig,
		h.Cfg.EnvCfg.VeleroBackup.Namespace,
	)
	if err != nil {
		return err
	}

	err = backup.Create(env.Namespace, veleroBackupName, veleroBackupTTL)
	if err != nil {
		return err
	}

	return nil
}
