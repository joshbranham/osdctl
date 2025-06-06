## osdctl dynatrace

Dynatrace related utilities

### Options

```
  -h, --help   help for dynatrace
```

### Options inherited from parent commands

```
      --as string                        Username to impersonate for the operation. User could be a regular user or a service account in a namespace.
      --cluster string                   The name of the kubeconfig cluster to use
      --context string                   The name of the kubeconfig context to use
      --insecure-skip-tls-verify         If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
  -o, --output string                    Valid formats are ['', 'json', 'yaml', 'env']
      --request-timeout string           The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                    The address and port of the Kubernetes API server
      --skip-aws-proxy-check aws_proxy   Don't use the configured aws_proxy value
  -S, --skip-version-check               skip checking to see if this is the most recent release
```

### SEE ALSO

* [osdctl](osdctl.md)	 - OSD CLI
* [osdctl dynatrace dashboard](osdctl_dynatrace_dashboard.md)	 - Get the Dyntrace Cluster Overview Dashboard for a given MC or HCP cluster
* [osdctl dynatrace gather-logs](osdctl_dynatrace_gather-logs.md)	 - Gather all Pod logs and Application event from HCP
* [osdctl dynatrace logs](osdctl_dynatrace_logs.md)	 - Fetch logs from Dynatrace
* [osdctl dynatrace url](osdctl_dynatrace_url.md)	 - Get the Dynatrace Tenant URL for a given MC or HCP cluster

