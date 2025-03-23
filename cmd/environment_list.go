package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/fragpit/env-cleaner/internal/api"
	"github.com/fragpit/env-cleaner/internal/model"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List environments",
	Long:    `List environments`,
	Run: func(cmd *cobra.Command, args []string) { //nolint:revive
		if err := List(); err != nil {
			log.Fatalf("Error: %v", err)
		}
	},
}

func init() {
	envCmd.AddCommand(listCmd)
}

func List() error {
	baseURL, err := url.Parse(cfg.APIURL)
	if err != nil {
		return fmt.Errorf("error parsing url: %w", err)
	}

	baseURL.Path = path.Join(baseURL.Path, apiEnvironmentsEndpoint)
	fullURL := baseURL.String()

	req, err := http.NewRequest(http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	encodedAPIKey := base64.StdEncoding.EncodeToString([]byte(cfg.AdminAPIKey))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedAPIKey))

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
			"failed to list environments: %s (code: %d)",
			resp.Error.Message,
			resp.Error.Code,
		)
	}

	var environments []model.Environment

	data, err := json.Marshal(resp.Data)
	if err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	if err := json.Unmarshal(data, &environments); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Owner\tID\tName\tType\tDeleteAt")
	for _, env := range environments {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			env.Owner, env.EnvID, setName(&env), env.Type, env.DeleteAt)
	}
	w.Flush()

	return nil
}
