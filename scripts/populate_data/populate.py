import json
import os
import requests
from typing import Dict, List, Union

# Base URL for your API
BASE_URL = "http://localhost:8080/api/v1"  # Adjust this to your actual API base URL

# Bearer token for authorization
BEARER_TOKEN = "<YOUR_BEARER>"

# Headers for the API requests
HEADERS = {
    "Authorization": f"Bearer {BEARER_TOKEN}",
    "Content-Type": "application/json"
}

# Entity types and their corresponding endpoints
ENTITIES = {
    "organization": "/organizations",
    "department": "/departments",
    "permission": "/permissions",
    "role": "/roles",
    "group": "/groups",
    "user": "/users",
    "resource": "/resources",
    "policy": "/policies",
    "resourceType": "/resource-types",
    "attributeGroup": "/attribute-groups",
}

def read_json_file(file_path: str) -> Union[Dict, List[Dict]]:
    """Read a JSON file and return its contents."""
    with open(file_path, 'r') as file:
        return json.load(file)

def create_entity(entity_type: str, data: Dict) -> None:
    """Create a single entity using the API."""
    url = f"{BASE_URL}{ENTITIES[entity_type]}"
    response = requests.post(url, json=data, headers=HEADERS)
    if response.status_code == 201:
        print(f"Successfully created {entity_type}: {data.get('id', 'N/A')}")
    else:
        print(f"Failed to create {entity_type}: {data.get('id', 'N/A')}")
        print(f"Status code: {response.status_code}")
        print(f"Response: {response.text}")

def populate_data(entity_type: str) -> None:
    """Populate data for a given entity type."""
    file_path = f"./data/{entity_type}.json"
    if not os.path.exists(file_path):
        print(f"File not found: {file_path}")
        return

    data = read_json_file(file_path)
    if isinstance(data, list):
        for item in data:
            create_entity(entity_type, item)
    else:
        create_entity(entity_type, data)

def main():
    """Main function to populate all entities."""
    # Order of creation is important due to dependencies
    creation_order = [
        "organization",
        "department",
        "permission",
        "role",
        "group",
        "user",
        "resourceType",
        "attributeGroup",
        "resource",
        "policy",
    ]

    for entity_type in creation_order:
        print(f"\nPopulating {entity_type}...")
        populate_data(entity_type)

if __name__ == "__main__":
    main()