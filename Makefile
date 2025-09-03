run:
	sudo go run .

build:
	sudo go build -o ignis-vm .
	@echo "Build complete. You can run the application with ./ignis-vm"

clean:
	./clean.sh
	@echo "Cleaned up the project."

.PHONY: build-rootfs

build-rootfs:
	cd agent-create && sudo ./build-all.sh