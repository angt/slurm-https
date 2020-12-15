FROM tetafro/golang-gcc as installer

RUN apk update && apk add git

RUN cd src \
    && git clone https://github.com/angt/slurm-https \
    && git clone https://github.com/SchedMD/slurm \
    && pwd

FROM installer as configer

RUN mkdir -p /src/slurm && mkdir -p /src/slurm-https

# COPY --from=installer /src/slurm-https /src/slurm-https
COPY --from=installer /go/src/slurm /src/slurm
COPY --from=installer /go/src/slurm-https /src/slurm-https


COPY slurm.pc /
