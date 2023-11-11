FROM dockcross/windows-static-x86

RUN apt-get update && \
    wget https://go.dev/dl/go1.24.1.linux-amd64.tar.gz && \
    tar -xvf go1.24.1.linux-amd64.tar.gz && \
    mv go /usr/local && \
    export GOROOT=/usr/local/go && \
    export GOPATH=$HOME/go && \
    export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

CMD [ "/bin/sh" ]
