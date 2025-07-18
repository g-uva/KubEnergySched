import argparse
from datetime import datetime, timezone, timedelta
from fabrictestbed_extensions.fablib.fablib import FablibManager
from time import sleep

def create_ipv4_public_slice(slice_name, ssh_key_name, lease_days):
    fablib = FablibManager()
    
    # Lease start/end times (timezone-aware UTC)
    start = datetime.now(timezone.utc)
    end = start + timedelta(days=lease_days)
    
    print(f"Creating slice: {slice_name}")
    slice = fablib.new_slice(name=slice_name)
    # NOTE: default reservation window is auto-assigned; custom lease timeline not supported in this API version

    # Attach SSH key by name (pre-uploaded in FABRIC portal)
    slice.add_key(ssh_key_name)

    # Hardcoded site and image
    site = "WASH"
    image = "default_ubuntu_20"
    flavor = "default"
    node_name = "ipv4-node"
    iface_name = "shared-nic"
    net_name = "public-proxy-net"

    # Add a node
    node = slice.add_node(name=node_name, site=site, image=image, flavor=flavor)
    node.add_key(ssh_key_name)
    iface = node.add_component(model="NIC_Basic", name=iface_name)

    # Create an L3 network (public IPv4 via FABNetv4) and bind interface
    net = slice.add_l3network(name=net_name, type="FABNetv4")
    net.add_interface(iface)

    # Submit slice and wait
    slice.submit()
    print("Waiting for slice to be ready...")
    slice.wait_while_creating()
    print("Slice is ready!")

    # Display SSH command
    ssh_cmd = node.get_ssh_command()
    print(f"SSH into the node using:\n{ssh_cmd}")

    return slice

if __name__ == "__main__":
    parser = argparse.ArgumentParser(
        description="Create FABRIC slice with IPv4 public access and NIC"
    )
    parser.add_argument(
        "-n", "--slice-name",
        dest="slice_name",
        required=True,
        help="Name of the slice to create"
    )
    parser.add_argument(
        "-k", "--ssh-key",
        dest="ssh_key_name",
        required=True,
        help="Name of the SSH key in FABRIC portal"
    )
    parser.add_argument(
        "-d", "--lease-days",
        dest="lease_days",
        type=int,
        default=5,
        help="Number of days for slice lease"
    )
    args = parser.parse_args()

    create_ipv4_public_slice(
        args.slice_name,
        args.ssh_key_name,
        args.lease_days
    )
