package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/fragpit/env-cleaner/internal/api"
	"github.com/fragpit/env-cleaner/internal/model"
)

var (
	envOwner     string
	envName      string
	envNamespace string
	envType      string
	envTTL       string
)

var addCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"create"},
	Short:   "Add environment",
	Long:    `Add environment`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := Add(cmd, args); err != nil {
			log.Fatalf("Error: %v", err)
		}
	},
}

func init() {
	envCmd.AddCommand(addCmd)

	addCmd.Flags().StringVarP(&envName, "name", "n", "", "Environment name")
	addCmd.Flags().
		StringVar(&envNamespace, "namespace", "", "Environment namespace for type helm")
	addCmd.Flags().StringVarP(&envOwner, "owner", "o", "", "Environment owner")
	addCmd.Flags().
		StringVarP(&envType, "type", "t", "", "Environment type (vsphere_vm, helm)")
	addCmd.Flags().
		StringVarP(&envTTL, "ttl", "", "", "Time to live for the environment")

	if err := addCmd.MarkFlagRequired("name"); err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := addCmd.MarkFlagRequired("owner"); err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := addCmd.MarkFlagRequired("type"); err != nil {
		log.Fatalf("Error: %v", err)
	}
	if err := addCmd.MarkFlagRequired("ttl"); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

// TODO: rework empty params
func Add(_ *cobra.Command, _ []string) error {
	env := api.Environment{
		Name:      envName,
		Namespace: envNamespace,
		Owner:     envOwner,
		Type:      envType,
		TTL:       envTTL,
	}

	if env.Type == "helm" && env.Namespace == "" {
		return fmt.Errorf("namespace parameter is required for helm type")
	}

	baseURL, err := url.Parse(cfg.APIURL)
	if err != nil {
		return fmt.Errorf("error parsing url: %w", err)
	}

	baseURL.Path = path.Join(baseURL.Path, apiEnvironmentsEndpoint)
	fullURL := baseURL.String()

	jsonData, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("error marshaling json: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fullURL,
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	encodedAPIKey := base64.StdEncoding.EncodeToString([]byte(cfg.AdminAPIKey))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedAPIKey))
	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer res.Body.Close()

	var resp api.Response
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf(
			"failed to add environment: %s (code: %d)",
			resp.Error.Message,
			resp.Error.Code,
		)
	}

	var environment model.Environment
	data, err := json.Marshal(resp.Data)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if err := json.Unmarshal(data, &environment); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	fmt.Printf(
		"Environment: %s id: %s type: %s added successfully",
		setName(&environment),
		environment.EnvID,
		environment.Type,
	)

	return nil
}
