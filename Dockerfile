FROM openeuler/openeuler:23.03 as BUILDER
RUN dnf update -y && \
    dnf install -y golang && \
    go env -w GOPROXY=https://goproxy.cn,direct

MAINTAINER zengchen1024<chenzeng765@gmail.com>

# build binary
WORKDIR /go/src/github.com/opensourceways/robot-gitee-access
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 go build -a -o robot-gitee-access -buildmode=pie --ldflags "-s -linkmode 'external' -extldflags '-Wl,-z,now'" .

# copy binary config and utils
FROM openeuler/openeuler:22.03
RUN dnf -y update && \
    dnf in -y shadow && \
    dnf remove -y gdb-gdbserver && \
    groupadd -g 1000 access && \
    useradd -u 1000 -g access -s /sbin/nologin -m access && \
    echo > /etc/issue && echo > /etc/issue.net && echo > /etc/motd && \
    mkdir /home/access -p && \
    chmod 700 /home/access && \
    chown access:access /home/access && \
    echo 'set +o history' >> /root/.bashrc && \
    sed -i 's/^PASS_MAX_DAYS.*/PASS_MAX_DAYS   90/' /etc/login.defs && \
    rm -rf /tmp/*

USER access

WORKDIR /opt/app

COPY  --chown=access --from=BUILDER /go/src/github.com/opensourceways/robot-gitee-access/robot-gitee-access /opt/app/robot-gitee-access

RUN chmod 550 /opt/app/robot-gitee-access && \
    echo "umask 027" >> /home/access/.bashrc && \
    echo 'set +o history' >> /home/access/.bashrc

ENTRYPOINT ["/opt/app/robot-gitee-access"]
