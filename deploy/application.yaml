apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: barbican-proxy
  namespace: argocd
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: barbican-proxy

  sources:
    - repoURL: "https://github.com/Mobility-Scooter-Project/mobility-scooter-infra.git"
      targetRevision: main
      path: charts/api
      helm:
        values: |
          name: barbican-proxy
          replicas: 1

          image:
            repository: ghcr.io/mobility-scooter-project/barbican-proxy/proxy
            tag: latest
            pullPolicy: IfNotPresent
            # If GHCR is private, make sure the namespace has a matching imagePullSecret.

          service:
            port: 8080
            targetPort: 3000

          ingress:
            enabled: true
            subdomain: proxy
            prefix: "/"
            useRootPath: true

          resources:
            requests:
              cpu: "250m"
              memory: "256Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"

          # Mount secret-based env (created from your env.proxy file)
          envFrom:
            - secretRef:
                name: barbican-proxy-secrets

          # Optional health checks (enable if the chart supports these keys)
          probes:
            enabled: true
            path: /health
            initialDelaySeconds: 5
            periodSeconds: 10

  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true