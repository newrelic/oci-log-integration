import json
import sys
import os
import requests

# This function witll read the S3 pre signed url and parse the JSON object
# The JSON object will be passed to terraform to create resources.
def main():
    try:
        # Read input from stdin 
        input_data = json.load(sys.stdin)
        
        # Get payload URL from Terraform input
        payload_link = input_data.get("payload_link")
        
        if not payload_link:
            sys.stderr.write("Error: payload_link not provided in input\n")
            sys.exit(1)
        
        # Fetch the payload from URL
        try:
            response = requests.get(payload_link)
            response.raise_for_status()
            payload_data = response.json()
        except (requests.RequestException, ValueError) as e:
            sys.stderr.write(f"Error fetching or parsing payload from URL: {e}\n")
            sys.exit(1)
        
        required_top_fields = ["nr_compartment_id", "ingest_key_vault_ocid", "connectors"]
        for field in required_top_fields:
            if field not in payload_data:
                sys.stderr.write(f"Error: '{field}' is missing from payload\n")
                sys.exit(1)

        connectors = payload_data.get("connectors", [])
        compartment_id = payload_data.get("nr_compartment_id", "")
        home_secret_ocid = payload_data.get("ingest_key_vault_ocid", "")
        
        # Output the processed data as a JSON object
        output_payload = {
            "connectors": json.dumps(connectors),
            "compartment_id": str(compartment_id),
            "home_secret_ocid": str(home_secret_ocid)
        }

        json.dump(output_payload, sys.stdout)
        
    except Exception as e:
        sys.stderr.write(f"Error: {str(e)}\n")
        sys.exit(1)

if __name__ == "__main__":
    main()