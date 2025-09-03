#!/bin/bash

echo "Select an option to clear:"
echo "1) Clear /tmp"
echo "2) Clear /var/lib/cni"
echo "3) Clear both"
read -p "Enter your choice [1-3]: " choice

case $choice in
  1)
    echo "Clearing /tmp directory..."
    sudo rm -rf /tmp
    echo "/tmp cleared."
    ;;
  2)
    echo "Clearing /var/lib/cni directory..."
    sudo rm -rf /var/lib/cni/*
    echo "/var/lib/cni cleared."
    ;;
  3)
    echo "Clearing both /tmp and /var/lib/cni directories..."
    sudo rm -rf /tmp
    sudo rm -rf /var/lib/cni/*
    echo "Both directories cleared."
    ;;
  *)
    echo "Invalid choice. Exiting."
    exit 1
    ;;
esac

echo "Done."
