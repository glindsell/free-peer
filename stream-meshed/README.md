# Helloworld-app
## Description  
This app demonstrates a GRPC server and client communicating in a single node k8s cluster, with load balancing applied between three pods using Linkerd.

## Run the app
Follow the instructions to install minikube:  
https://kubernetes.io/docs/tasks/tools/install-minikube/

Start minikube:  
```minikube start```

Set docker env:  
```eval $(minikube docker-env)```

Build docker image:  
```docker build -t helloworld-app .```

Run local image as client in minikube:  
```kubectl run helloworld-app-client --image=helloworld-app:latest --port=50051 --image-pull-policy=Never```

Check that it's running:  
```kubectl get pods```

Install Linkerd:  
```curl -sL https://run.linkerd.io/install | sh```

Add linkerd to your path:  
```export PATH=$PATH:$HOME/.linkerd2/bin```

Verify the CLI is installed and running correctly:  
```linkerd version```

To check that your cluster is configured correctly and ready to install the control plane, you can run:  
```linkerd check --pre```

Install the lightweight control plane into its own namespace (linkerd):  
```linkerd install | kubectl apply -f -```

Validate that everything’s happening correctly:  
(This command will patiently wait until Linkerd has been installed and is running.)  
```linkerd check```

In a new terminal view the Linkerd dashboard by running:  
```linkerd dashboard```

Back in terminal view deployments:  
```kubectl -n linkerd get deploy```

Install the app as a new deployment (using docker hub image at glindsell/helloworld-app):  
```linkerd inject hello-world-grpc.yml | kubectl apply -f -```

View new deployment in the Linkerd dashboard.  

Expose the deployment as a service:  
```kubectl expose deployment helloworld-app-server --type=NodePort```

Get IP Address of services:  
```kubectl get services```

Exec into client:  
```kubectl exec -it helloworld-app-client-<id-goes-here> -- /bin/bash```

cd into app dir:  
```cd src/helloworld-app/```

Change <IP-ADDR> to CLUSTER-IP of helloworld-app-server (line 32 of helloworld-app/greeter_client):  
```vim greeter_client/main.go```  
change:  
```address = "<IP-ADDR>:50051"```

Run the client to send requests:  
```bash request.sh```

View load balancing occuring accross all three helloworld-app-server pods.  

Click on Grafana icon to view individual pod’s stats.  

To bring everything down:  
```minikube stop```  
```minikube delete```
