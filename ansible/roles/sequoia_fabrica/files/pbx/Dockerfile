FROM ubuntu:24.04
# Ubuntu's Asterisk 20 LTS: PJSIP, AudioSocket and func_curl, nothing else to build
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      asterisk asterisk-modules asterisk-core-sounds-en asterisk-core-sounds-en-gsm asterisk-moh-opsound-gsm ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && rm -rf /etc/asterisk/* \
 && install -d -o asterisk -g asterisk /var/run/asterisk
USER asterisk
ENTRYPOINT ["/usr/sbin/asterisk", "-f", "-vvv", "-g"]
