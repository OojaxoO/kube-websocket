BINARY_NAME=ops-push
INSTALL_DIR=/opt/ops-push/

build:
	go get
	go build -o ${BINARY_NAME} 
install:
	install -D ${BINARY_NAME} ${INSTALL_DIR}/${BINARY_NAME}
	cp config.yaml /opt/ops-push/ 
