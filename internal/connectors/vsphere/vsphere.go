package vsphere

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/fragpit/env-cleaner/internal/config"
	"github.com/fragpit/env-cleaner/internal/model"
	"github.com/fragpit/env-cleaner/pkg/utils"
)

const connectorType = "vsphere_vm"

type Connector struct {
	Config
	Client      *govmomi.Client
	Notificator model.Notificator
}

type Config struct {
	EnvCfg  config.VSphereVM
	ConnCfg config.VSphere
}

var _ model.Connector = (*Connector)(nil)

func New(
	ctx context.Context,
	cfg *Config,
	nt model.Notificator,
) (*Connector, error) {
	// https://administrator@vsphere.local:pass1234@vcenter.devlab/sdk
	pass := url.QueryEscape(cfg.ConnCfg.Password)
	vSphereURL := fmt.Sprintf("https://%s:%s@%s/sdk", cfg.ConnCfg.Username,
		pass, cfg.ConnCfg.Hostname)

	u, err := url.Parse(vSphereURL)
	if err != nil {
		return nil, fmt.Errorf("error creating connector: %w", err)
	}

	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, fmt.Errorf("error creating connector: %w", err)
	}

	return &Connector{
		Client:      client,
		Config:      *cfg,
		Notificator: nt,
	}, nil
}

func (vc *Connector) validateSession(ctx context.Context) error {
	if s, _ := vc.Client.SessionManager.UserSession(ctx); s == nil {
		// https://administrator@vsphere.local:pass1234@vcenter.devlab/sdk
		pass := url.QueryEscape(vc.ConnCfg.Password)
		vSphereURL := fmt.Sprintf("https://%s:%s@%s/sdk", vc.ConnCfg.Username,
			pass, vc.ConnCfg.Hostname)

		u, _ := url.Parse(vSphereURL)

		if err := vc.Client.Login(ctx, u.User); err != nil {
			return fmt.Errorf("error validating : %w", err)
		}
	}

	return nil
}

func (vc *Connector) GetEnvironments(
	ctx context.Context,
) ([]model.Environment, error) {
	if err := vc.validateSession(ctx); err != nil {
		return nil, fmt.Errorf("error finding vms: %w", err)
	}

	finder := find.NewFinder(vc.Client.Client, false)

	dc, err := finder.Datacenter(ctx, vc.ConnCfg.Datacenter)
	if err != nil {
		return nil, fmt.Errorf("error finding vms: %w", err)
	}
	finder.SetDatacenter(dc)

	var foundVMs []*object.VirtualMachine
	for _, folder := range vc.EnvCfg.WatchFolders {
		folderName := "/" + vc.ConnCfg.Datacenter + "/vm/" + folder + "/"

		_, err := finder.Folder(ctx, folderName)
		if err != nil {
			log.Error("Folder not found: ", folderName)
			continue
		}

		var notFoundError *find.NotFoundError
		vms, err := finder.VirtualMachineList(ctx, folderName+"*")
		if errors.As(err, &notFoundError) {
			log.Infof("No virtual machines found in folder: %s", folderName)
		} else if err != nil {
			return nil, fmt.Errorf("error finding vms: %w", err)
		}

		foundVMs = append(foundVMs, vms...)
	}

	if len(foundVMs) == 0 {
		return nil, nil
	}

	pc := property.DefaultCollector(vc.Client.Client)

	refs := make([]types.ManagedObjectReference, 0, len(foundVMs))
	for _, vm := range foundVMs {
		refs = append(refs, vm.Reference())
	}

	var vmt []mo.VirtualMachine
	err = pc.Retrieve(ctx, refs, []string{"name", "summary"}, &vmt)
	if err != nil {
		return nil, fmt.Errorf("error finding vms: %w", err)
	}

	env := make([]model.Environment, 0, len(vmt))
	for i := range vmt {
		vm := &vmt[i]
		if slices.Contains(vc.EnvCfg.BlacklistVMs, vm.Name) {
			log.Warnf("Skipped VM: %s: blacklisted", vm.Name)
			continue
		}

		owner := parseAnnotation(vm.Summary.Config.Annotation, "EC_OWNER")
		ttl := parseAnnotation(vm.Summary.Config.Annotation, "EC_TTL")
		if owner == "" || ttl == "" {
			log.Warnf("Skipped VM %s: owner or ttl is empty", vm.Name)
			envTmp := &model.Environment{
				Name: vm.Name,
				Type: connectorType,
			}
			if err := vc.Notificator.SendOrphanMessage(envTmp); err != nil {
				return nil, fmt.Errorf("error processing: %w", err)
			}
			continue
		}

		deleteAt, deleteAtSec, err := utils.SetDeleteAt(ttl)
		if err != nil {
			log.Warnf("Skipped VM %s: error setting delete_at: %v", vm.Name, err)
			continue
		}

		env = append(env, model.Environment{
			EnvID:       vm.ManagedEntity.ExtensibleManagedObject.Self.Value,
			Type:        connectorType,
			Name:        vm.Name,
			Namespace:   "",
			Owner:       owner,
			DeleteAt:    deleteAt,
			DeleteAtSec: deleteAtSec,
		})
	}

	return env, nil
}

func (vc *Connector) GetEnvironmentID(
	ctx context.Context,
	env *model.Environment,
) (string, error) {
	if err := vc.validateSession(ctx); err != nil {
		return "", fmt.Errorf("error get env id: %w", err)
	}

	if env.Name == "" {
		return "", fmt.Errorf("error get env id: %w", errors.New("name is empty"))
	}

	vmID, err := vc.searchVMbyName(ctx, env.Name)
	if err != nil {
		return "", fmt.Errorf("error get env id: %w", err)
	}

	return vmID, nil
}

func (vc *Connector) DeleteEnvironment(
	ctx context.Context,
	env *model.Environment,
) error {
	if err := vc.validateSession(ctx); err != nil {
		return fmt.Errorf("error delete env: %w", err)
	}

	finder := find.NewFinder(vc.Client.Client, false)

	vmMOR := types.ManagedObjectReference{
		Type:  "VirtualMachine",
		Value: env.EnvID,
	}

	if _, err := finder.ObjectReference(ctx, vmMOR); err != nil {
		return fmt.Errorf("error quarantine vm: %w", err)
	}

	vm := object.NewVirtualMachine(vc.Client.Client, vmMOR)
	_, err := vm.PowerOff(ctx)
	if err != nil {
		return fmt.Errorf("error quarantine vm: %w", err)
	}

	if vc.EnvCfg.QuarantineFolderID != "" {
		vmrs := types.VirtualMachineRelocateSpec{
			Folder: &types.ManagedObjectReference{
				Type:  "Folder",
				Value: vc.EnvCfg.QuarantineFolderID,
			},
		}
		if _, err := vm.Relocate(ctx, vmrs, "defaultPriority"); err != nil {
			return fmt.Errorf("error quarantine vm: %w", err)
		}
	}

	if vc.EnvCfg.QuarantinePostfix != "" {
		vmRenamedName := env.Name + vc.EnvCfg.QuarantinePostfix + "-" + time.Now().
			Format("20060102150405")
		if _, err := vm.Rename(ctx, vmRenamedName); err != nil {
			return fmt.Errorf("error quarantine vm: %w", err)
		}
	}

	return nil
}

func (vc *Connector) CheckEnvironment(
	ctx context.Context,
	env *model.Environment,
) error {
	if err := vc.validateSession(ctx); err != nil {
		return fmt.Errorf("error check env: %w", err)
	}

	finder := find.NewFinder(vc.Client.Client, false)

	if env.EnvID != "" {
		vmMOR := types.ManagedObjectReference{
			Type:  "VirtualMachine",
			Value: env.EnvID,
		}

		if _, err := finder.ObjectReference(ctx, vmMOR); err != nil {
			return fmt.Errorf("error finding vm: %w", err)
		}
	} else if env.Name != "" {
		_, err := vc.searchVMbyName(ctx, env.Name)
		if err != nil {
			return fmt.Errorf("error finding vm: %w", err)
		}
	} else {
		return fmt.Errorf(
			"error check env: %w", errors.New("env_id or name is empty"),
		)
	}

	return nil
}

func (vc *Connector) GetConnectorType() string {
	return connectorType
}

func parseAnnotation(annotation, key string) string {
	lines := strings.Split(annotation, "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func (vc *Connector) searchVMbyName(
	ctx context.Context,
	vmName string,
) (string, error) {
	finder := find.NewFinder(vc.Client.Client, false)

	dc, err := finder.Datacenter(ctx, vc.ConnCfg.Datacenter)
	if err != nil {
		return "", fmt.Errorf("error finding vm: %w", err)
	}
	finder.SetDatacenter(dc)

	m := view.NewManager(vc.Client.Client)
	root := vc.Client.Client.ServiceContent.RootFolder
	v, err := m.CreateContainerView(ctx, root, []string{"VirtualMachine"}, true)
	if err != nil {
		return "", fmt.Errorf("error finding vm: %w", err)
	}

	defer func() {
		_ = v.Destroy(ctx)
	}()

	filter := property.Match{"name": vmName}
	objs, err := v.Find(ctx, []string{"VirtualMachine"}, filter)
	if err != nil {
		return "", fmt.Errorf("error finding vm: %w", err)
	}

	if len(objs) == 0 {
		return "", fmt.Errorf("error finding vm: %w", errors.New("vm not found"))
	} else if len(objs) > 1 {
		return "", fmt.Errorf("error finding vm: %w", errors.New("multiple vms found"))
	}

	vmID := objs[0].Value

	return vmID, nil
}
