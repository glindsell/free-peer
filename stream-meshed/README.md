# Helloworld-app
## Description  
This app demonstrates a GRPC server and client communicating in a multi node k8s cluster, with load balancing applied between three pods using Linkerd.

## Run the app
1. Bring up cluster:  
```vagrant up```

2. Ssh into k8s-head:  
```vagrant global-status```

3. Build docker image:  
```vagrant ssh <vagrant-machine-id>```

4. Install Linkerd:  
```curl -sL https://run.linkerd.io/install | sh```

5. Add linkerd to your path:  
```export PATH=$PATH:$HOME/.linkerd2/bin```
(You must do this in each new vagrant ssh instance - or add the line to .bashrc in the vm)

6. Verify the CLI is installed and running correctly:  
```linkerd version```

7. To check that your cluster is configured correctly and ready to install the control plane, you can run:  
```linkerd check --pre```

8. Install the lightweight control plane into its own namespace (linkerd):  
```linkerd install | kubectl apply -f -```

9. Validate that everything’s happening correctly:  
(This command will patiently wait until Linkerd has been installed and is running.)  
```linkerd check```

10. In a new terminal ssh into k8s-head and view the Linkerd dashboard by running:  
```linkerd dashboard```
(You will need to forward the port shown on the vm to access the dashboard from the host - this can be done without restarting the vm by opening the virtualbox gui Settings -> Network -> Port Fordwarding)

11. Inject the ingress yaml files:  
```
cd ingress
linkerd inject nginx-ingress.yaml | kubectl apply -f -
linkerd inject nginx-ingress-svc.yaml | kubectl apply -f -
```

12. Clone the project and checkout the correct branch:  
```
cd
mkdir -p go/src/github.com/chainforce/free-peer
cd go/src/github.com/chainforce/free-peer/
git clone https://github.com/glindsell/free-peer.git
cd free-peer/
git fetch --all
git checkout ingress
```

13. Inject the gRPC servers:  
```
cd stream-meshed/
linkerd inject hw_server.yml | kubectl apply -f -
```

14. Inject Ingress:  
```
cd ~/ingress/
linkerd inject ingress.yml | kubectl apply -f -
```

15. View node port for nginx-ingress:  
```
kubectl get svc nginx-ingress -o wide -n ingress-nginx
```

16. Run linkerd tap to view messages hitting the ingress
```
linkerd tap pod/nginx-ingress-controller-69699fdffd-n2x4f -n ingress-nginx
```
(id of ingress controller will vary between deployments)

17. Repeat step 12 on the host.  

18. Edit gRPC client:  
```vim greeter_client/main.go```
And change port in line 13 to match node port for nginx-ingress
change:  
```address = "ingress.local:<NODEPORT>"```

19. Add: `127.0.0.1 ingress.local` to your /etc/hosts

20. Run gRPC client:
```
go run greeter_client/main.go
```

To bring everything down:  
```vagrant destroy -f```  
