# TODO @ksarina256
This is where your ArgoCD application YAML file should go. Things to consider:
- which helm chart does this fall under?
- what env vars do I need to pass to it?
- how much CPU and RAM does this app need?
The [model-service](https://github.com/Mobility-Scooter-Project/model-service/blob/main/deploy/applicationset.yaml) is a good place to start, although keep in mind that it is an applicationset, eg designed to create multiple apps, but you only need one for this repo.