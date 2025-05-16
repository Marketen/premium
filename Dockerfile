FROM busybox
  
  WORKDIR /usr/src/app
  
  ENTRYPOINT while true; do echo "happy build $USERNAME!"; sleep 1; done
  
  