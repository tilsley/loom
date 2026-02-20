package main

import "fmt"

type envCfg struct {
	replicas string
	tag      string
}

var appEnvs = map[string]map[string]envCfg{
	"billing-api": {
		"dev":     {replicas: "1", tag: "dev-latest"},
		"staging": {replicas: "2", tag: "v1.2.0"},
		"prod":    {replicas: "3", tag: "v1.1.0"},
	},
	"user-service": {
		"dev":     {replicas: "1", tag: "dev-latest"},
		"staging": {replicas: "2", tag: "v2.0.0-rc1"},
		"prod":    {replicas: "3", tag: "v1.9.0"},
	},
}

// seedRepos populates the file store with initial content for all repos.
// Called during init before the server accepts requests.
func seedRepos(s *store) {
	for _, app := range []string{"billing-api", "user-service"} {
		seedGitopsApp(s, app)
		seedAppRepo(s, app)
	}
}

func seedGitopsApp(s *store, app string) {
	const repo = "acme/gitops"
	if s.files[repo] == nil {
		s.files[repo] = make(map[string]string)
	}

	s.files[repo][fmt.Sprintf("apps/%s/base/application.yaml", app)] = baseApplication(app)
	s.files[repo][fmt.Sprintf("apps/%s/base/service-monitor.yaml", app)] = serviceMonitor(app)

	for env, cfg := range appEnvs[app] {
		s.files[repo][fmt.Sprintf("apps/%s/overlays/%s/application.yaml", app, env)] =
			envApplication(app, env, cfg.replicas, cfg.tag)
	}
}

func seedAppRepo(s *store, app string) {
	repoKey := "acme/" + app
	s.files[repoKey] = map[string]string{
		".github/workflows/ci.yaml": ciWorkflow(),
	}
}

func baseApplication(app string) string {
	return fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://charts.example.com/generic
    chart: generic-app
    targetRevision: 1.0.0
    helm:
      values: |
        nameOverride: %s
        image:
          repository: acme/%s
        serviceMonitor:
          enabled: true
  destination:
    server: https://kubernetes.default.svc
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
`, app, app, app)
}

func envApplication(app, env, replicas, tag string) string {
	return fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s-%s
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://charts.example.com/generic
    chart: generic-app
    targetRevision: 1.0.0
    helm:
      parameters:
        - name: replicaCount
          value: "%s"
        - name: image.tag
          value: "%s"
        - name: namespace
          value: "%s"
      values: |
        nameOverride: %s
        image:
          repository: acme/%s
        serviceMonitor:
          enabled: true
  destination:
    server: https://kubernetes.default.svc
    namespace: %s
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
`, app, env, replicas, tag, env, app, app, env)
}

func serviceMonitor(app string) string {
	return fmt.Sprintf(`apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: %s
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: %s
  endpoints:
    - port: metrics
      interval: 30s
`, app, app)
}

func ciWorkflow() string {
	return `name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build
        run: make build
      - name: Test
        run: make test
`
}
