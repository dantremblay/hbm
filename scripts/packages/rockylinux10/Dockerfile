FROM docker.io/rockylinux/rockylinux:10

COPY entrypoint.sh /entrypoint.sh
COPY build /usr/local/src/hbm

ENV SHELL=/bin/bash

WORKDIR /tmp

ENV RPMBUILD_PATH="/srv/rpmbuild"

RUN build="rpm-build systemd-rpm-macros" \
	&& set -x \
	&& yum -y clean all \
	&& yum -y --refresh install $build \
	&& yum -y autoremove \
	&& yum clean all

RUN mkdir -p ${RPMBUILD_PATH} \
	&& mkdir ${RPMBUILD_PATH}/BUILD \
	&& mkdir ${RPMBUILD_PATH}/RPMS \
	&& mkdir ${RPMBUILD_PATH}/SOURCES \
	&& mkdir ${RPMBUILD_PATH}/SPECS \
	&& mkdir ${RPMBUILD_PATH}/SRPMS \
	&& mkdir ${RPMBUILD_PATH}/tmp \
	&& echo "%_topdir  ${RPMBUILD_PATH}" > /root/.rpmmacros \
	&& echo "%_tmppath  ${RPMBUILD_PATH}/tmp" >> /root/.rpmmacros

COPY hbm.spec ${RPMBUILD_PATH}/SPECS/hbm.spec

RUN set -x \
	&& tar cvzf ${RPMBUILD_PATH}/SOURCES/hbm.tar.gz -C /usr/local/src hbm \
	&& rm -rf /usr/local/src/hbm

ENTRYPOINT ["/entrypoint.sh"]
