# propeller

Manages Helm charts running in Kubernetes clusters.

## Use Cases
### Managing multiple Helm charts
* Use declarative configurations to specify the versions of Helm charts running in your cluster.
* Easily override chart values and commit your changes to source control.
* Use charts from multiple Helm repos.

### Managing multiple Kubernetes clusters
* Use different charts and different versions in each cluster.
* Share chart overrides across clusters with a `default.yaml` file.
* Make cluster-specific chart overrides when necessary.

### Other features
* Use it as a [Drone](https://drone.io/) plugin for CI/CD.
* Read secrets from environment variables.

## How to use
### Command line
`propeller --cluster-config-path=./clusters/my-cluster.yaml --kube-config="$(cat ~/.kube/config)" --kube-context my-kubernetes-context`

### Drone pipeline
#### Simple example
This example Drone pipeline shows how to manage a single clusters. Updates are automatically deployed on a push/merge to master.

```yaml
deploy-charts:
  when:
    event: push
    branch: master
  image: path-to-docker/image:version
  cluster_config: clusters/my-cluster-name.yaml
  kube_context: my-kubernetes-context
  secrets:
    - source: my-kube-config-drone-secret
      target: KUBE_CONFIG
```

#### Multi-cluster example
This example demonstrates managing multiple clusters with a Drone matrix. Updates will be automatically deployed to test clusters when commit is pushed/merged to master. Production clusters can be deployed to manually by using a `drone deploy` command, allowing additional control over which versions reach production.

```yaml
matrix:
  include:
    - cluster: my-prod-cluster-1
      stage: prod
    - cluster: my-prod-cluster-2
      stage: prod
    - cluster: my-test-cluster-1
      stage: test
    - cluster: my-test-cluster-2
      stage: test

pipeline:
  deploy-charts-prod:
    when:
      event: deployment
      matrix:
        stage: prod
        cluster: ${DRONE_DEPLOY_TO}
    image: path-to-docker/image:version
    cluster_config: clusters/${cluster}.yaml
    kube_context: ${cluster}
    secrets:
      - source: my-kube-config-drone-secret
        target: KUBE_CONFIG

  deploy-charts-test:
    when:
      event: push
      branch: master
    image: path-to-docker/image:version
    cluster_config: clusters/${cluster}.yaml
    kube_context: ${cluster}
    secrets:
      - source: my-kube-config-drone-secret
        target: KUBE_CONFIG
```

## Files and Directory Layout
```
 chart-configs/
 |- clusters/
    |- my-cluster-name.yaml
    |- my-other-cluster-name.yaml
 |- values/
    |- cluster-autoscaler/            # the release name from your cluster file
       |- default.yaml                # overrides for all clusters
       |- my-cluster-name.yaml        # overrides for a specific cluster
       |- my-other-cluster-name.yaml
    |- my-chart/
       |- default.yaml
```

clusters/my-cluster-name.yaml:
```yaml
name: my-cluster-name  # This is used to find cluster-specific override files
helm:
  upgrade: true  # Should Tiller be auto-upgraded?
  repos:  # Make Helm aware of any repos you want to use
    - name: stable
      url: https://kubernetes-charts.storage.googleapis.com/
    - name: private-repo
      url: https://example.com/my-private-repo/
releases:
  - name: cluster-autoscaler  # Specify the release name
    namespace: kube-system  # Specify the namespace where to install
    version: 0.7.0  # Specify the version of the chart to install
    chartPath: stable/cluster-autoscaler
  - name: my-chart
    namespace: kube-system
    version: ~1.x  # Supports the same syntax as Helm's --version flag
    chartPath: private-repo/my-chart
```

values/my-chart/default.yaml:
```yaml
# Place any overrides here, just as you would with Helm.
# This file will be passed as an override to Helm.
resources:
  cpu:
    requests: 100m
    limits: 200m
  memory:
    requests: 1Gi
    limits: 1Gi
```
