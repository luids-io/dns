#!/bin/bash

## Configuration variables. 
RELEASE="v0.0.1"
ARCH="amd64"
SVC_USER=coredns
ETC_DIR=/etc/ludns
BIN_DIR=/usr/local/bin
SYSTEMD_DIR=/etc/systemd/system
DOWNLOAD_BASE="https://github.com/luids-io/dns/releases/download"
DOWNLOAD_URI="${DOWNLOAD_BASE}/${RELEASE}/ludns_${RELEASE}_linux_${ARCH}.tgz"
##

die() { echo "error: $@" 1>&2 ; exit 1 ; }

## some checks
for deps in "wget" "mktemp" "getent" "useradd" ; do
	which $deps >/dev/null \
		|| die "$deps is required!"
done
[[ $EUID -eq 0 ]] || die "This script must be run as root"
[ -d $BIN_DIR ] || die "Binary directory $BIN_DIR doesn't exist"

## options command line
OPT_UNATTEND=0
OPT_OVERWRITE_BIN=0
while [ -n "$1" ]; do
	case "$1" in
		-u) OPT_UNATTEND=1 ;;
		-o) OPT_OVERWRITE_BIN=1 ;; 
		-h) echo -e "Options:\n\t [-u] unattend\n\t [-o] overwrite binaries\n"
		    exit 0 ;; 
 		*) die "Option $1 not recognized" ;; 
	esac
	shift
done

echo
echo "==============="
echo "luDNS installer"
echo "==============="
echo

show_actions() {
	echo "Warning! This script will commit the following changes to your system:"
	echo ". Download and install binaries in '${BIN_DIR}'"
	echo ". Create a system user '${SVC_USER}'"
	echo ". Set capabilities to binary '${BIN_DIR}/ludns'"
	echo ". Create config dir '${ETC_DIR}'"
	[ -d $SYSTEMD_DIR ] && echo ". Copy systemd configurations to '${SYSTEMD_DIR}'"
	echo ""
}

if [ $OPT_UNATTEND -eq 0 ]; then
	show_actions
	read -p "Are you sure? (y/n) " -n 1 -r
	echo
	echo
	if [[ ! $REPLY =~ ^[Yy]$ ]]
	then
		die "canceled"
	fi
fi

TMP_DIR=$(mktemp -d -t ins-XXXXXX) || die "couldn't create temp"
LOG_FILE=${TMP_DIR}/installer.log

log() { echo `date +%y%m%d%H%M%S`": $@" >>$LOG_FILE ; }
step() { echo -n "* $@..." ; log "STEP: $@" ; }
step_ok() { echo " OK" ; }
step_err() { echo " ERROR" ; }
user_exists() { getent passwd $1>/dev/null ; }
group_exists() { getent group $1>/dev/null ; }

## do functions
do_download() {
	[ $# -eq 2 ] || die "${FUNCNAME}: unexpected number of params"
	local url="$1"
	local filename="$2"

	local dst="${TMP_DIR}/${filename}"
	rm -f $dst
	log "downloading $url"
	echo "$url" | grep -q "^\(http\|ftp\)"
	if [ $? -eq 0 ]; then
		wget "$url" -O $dst &>>$LOG_FILE
	else
		cp -v "$url" $dst &>>$LOG_FILE
	fi
}

do_clean_file() {
	[ $# -eq 1 ] || die "${FUNCNAME}: unexpected number of params"
	local filename=$1

	local src="${TMP_DIR}/${filename}"
	log "clearing $src"    
	rm -f $src &>>$LOG_FILE
}

do_install_bin() {
	[ $# -eq 1 ] || die "${FUNCNAME}: unexpected number of params"
	local binary=$1

	local src="${TMP_DIR}/${binary}"
	local dst="${BIN_DIR}/${binary}"
	[ ! -f $src ] && log "$src not found!" && return 1

	log "copying $src to $dst, chown root, chmod 755"
	{ cp $src $dst \
		&& chown root:root $dst \
		&& chmod 755 $dst
	} &>>$LOG_FILE
}

do_setcap_bind() {
	[ $# -eq 1 ] || die "${FUNCNAME}: unexpected number of params"
	local binary=$1

	local fpath="${BIN_DIR}/${binary}"
	[ ! -f $fpath ] && log "$fpath not found!" && return 1

	log "set bind capability to $fpath"
	setcap CAP_NET_BIND_SERVICE=+eip $fpath &>>$LOG_FILE
}

do_unpackage() {
	[ $# -eq 1 ] || die "${FUNCNAME}: unexpected number of params"
	local tgzfile=$1
	
	local src="${TMP_DIR}/${tgzfile}"
	[ ! -f $src ] && log "${FUNCNAME}: $src not found!" && return 1

	log "unpackaging $tgzfile"
	tar -zxvf $src -C $TMP_DIR &>>$LOG_FILE
}

do_create_datadir() {
	[ $# -eq 2 ] || die "${FUNCNAME}: unexpected number of params"
	local datadir=$1
	local datagrp=$2

	[ -d $datadir ] && log "$datadir found!" && return 1
	group_exists $datagrp || { log "group $datagrp doesn't exists" && return 1 ; }

	log "creating dir $datadir, chgrp to $datagrp, chmod g+s"
	{ mkdir -p $datadir \
		&& chown root:$datagrp $datadir \
		&& chmod 775 $datadir \
		&& chmod g+s $datadir
	} &>>$LOG_FILE
}

do_create_sysuser() {
	[ $# -eq 2 ] || die "${FUNCNAME}: unexpected number of params"
	local nuser=$1
	local nhome=$2

	user_exists $nuser && log "user $nuser already exists" && return 1

	log "useradd $nuser with params"
	useradd -s /usr/sbin/nologin -r -M -d $nhome $nuser &>>$LOG_FILE
}

## steps
install_binaries() {
	step "Downloading and installing binaries"
	if [ $OPT_OVERWRITE_BIN -eq 0 ]; then
		[ -f ${BIN_DIR}/ludns ] \
			&& log "${BIN_DIR}/ludns already exists" \
			&& step_ok && return 0
	fi

	do_download "$DOWNLOAD_URI" ludns_linux.tgz
	[ $? -ne 0 ] && step_err && return 1

	do_unpackage ludns_linux.tgz
	[ $? -ne 0 ] && step_err && return 1
	do_clean_file ludns_linux.tgz

	do_install_bin ludns
	[ $? -ne 0 ] && step_err && return 1
	do_clean_file ludns

	step_ok
}

set_capabilities() {
	step "Setting capabilities to binaries"
	{ chown root:${SVC_USER} $BIN_DIR/ludns \
	  && chmod 750 $BIN_DIR/ludns
	} &>>$LOG_FILE
	[ $? -ne 0 ] && step_err && return 1

	do_setcap_bind ludns
	[ $? -ne 0 ] && step_err && return 1

	step_ok
}

create_system_user() {
	step "Creating system user"
	user_exists $SVC_USER \
		&& log "user $SVC_USER already exists" && step_ok && return 0
	
	do_create_sysuser "$SVC_USER" "$ETC_DIR"
	[ $? -ne 0 ] && step_err && return 1
	
	step_ok
}

create_config() {
	step "Creating config dir with sample files"
	if [ ! -d $ETC_DIR ]; then
		log "creating dir $ETC_DIR"
		{ mkdir -p $ETC_DIR \
			&& chown root:root $ETC_DIR \
			&& chmod 755 $ETC_DIR
		} &>>$LOG_FILE
		[ $? -ne 0 ] && step_err && return 1
	else
		log "$ETC_DIR already exists"
	fi
	if [ ! -f $ETC_DIR/Corefile ]; then
		log "creating $ETC_DIR/Corefile"
		{ cat > $ETC_DIR/Corefile <<EOF
.:1053 {
    #health
    #prometheus :9153
    #idsevent
    #xlisthole
    #resolvarchive
    #resolvcache
    forward . 8.8.8.8 8.8.4.4
}
EOF
		} &>>$LOG_FILE
		[ $? -ne 0 ] && step_err && return 1
	else
		log "$ETC_DIR/Corefile already exists" && step_ok && return 0
	fi

	step_ok
}

install_systemd_services() {
	step "Installing systemd services"
	if [ ! -f $SYSTEMD_DIR/ludns.service ]; then
		log "creating $SYSTEMD_DIR/ludns.service"
		{ cat > $SYSTEMD_DIR/ludns.service <<EOF
[Unit]
Description=ludns service
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=on-failure
RestartSec=1
User=$SVC_USER
ExecStart=$BIN_DIR/ludns -conf $ETC_DIR/Corefile

[Install]
WantedBy=multi-user.target
EOF
		} &>>$LOG_FILE
		[ $? -ne 0 ] && step_err && return 1
	else
		log "$SYSTEMD_DIR/ludns.service already exists"
	fi
	step_ok
}



## main process

install_binaries || die "Show $LOG_FILE"
create_system_user || die "Show $LOG_FILE"
set_capabilities || die "Show $LOG_FILE"
create_config || die "Show $LOG_FILE"
[ -d $SYSTEMD_DIR ] && { install_systemd_services || die "Show $LOG_FILE for details." ; }

echo
echo "Installation success!. You can see $LOG_FILE for details."
