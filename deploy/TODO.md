apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: barbican-proxy
  namespace: argocd
  annotations:
    kargo.akuity.io/authorized-stage: "msp:barbican-proxy-deploy"
spec:
  destination:
    namespace: barbican-proxy
    server: "https://kubernetes.default.svc"
  project: default
  sources:
    - repoURL: "https://github.com/Mobility-Scooter-Project/mobility-scooter-infra.git"
      targetRevision: main
      path: charts/api
      helm:
        values: |
          name: barbican-proxy
          replicas: 1
          image:
            repository: ghcr.io/mobility-scooter-project/barbican-proxy
            tag: latest
            pullPolicy: IfNotPresent

          resources:
            requests:
              cpu: "250m"
              memory: "512Mi"
            limits:
              cpu: "500m"
              memory: "1Gi"

          ingress:
            enabled: true
            subdomain: barbican
            prefix: "/api/v1"
            useRootPath: true

          database:
            enabled: false

          kv:
            enabled: true
            url: "redis://redis-service:6379"

          queue:
            enabled: false

          service:
            port: 3000
            targetPort: 3000

          env:
            - name: OS_APPLICATION_CREDENTIAL_CLIENT_ID
              value: "a017bf17b9ab4a8097ef311eb3defd63"
            - name: OS_APPLICATION_CREDENTIAL_CLIENT_SECRET
              valueFrom:
                secretKeyRef:
                  name: barbican-proxy-secrets
                  key: os-client-secret
            - name: OS_AUTH_URL
              value: "https://js2.jetstream-cloud.org:5000/v3/"
            - name: BARBICAN_URL
              value: "https://js2.jetstream-cloud.org:9311"
            - name: KV_UR
              value: "redis://redis-service:6379"

  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true