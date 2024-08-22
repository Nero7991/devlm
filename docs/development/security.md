import os
from cryptography.fernet import Fernet
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.backends import default_backend
import base64
from dotenv import load_dotenv
import re

def use_environment_variables(sensitive_data):
    os.environ['SENSITIVE_DATA'] = sensitive_data
    return os.environ

def prevent_secret_commits(code_changes):
    patterns = [
        r'API_KEY\s*=\s*["\'].*["\']',
        r'SECRET_KEY\s*=\s*["\'].*["\']',
        r'PASSWORD\s*=\s*["\'].*["\']',
        r'ACCESS_TOKEN\s*=\s*["\'].*["\']'
    ]
    for pattern in patterns:
        if re.search(pattern, code_changes, re.IGNORECASE):
            raise ValueError("Potential sensitive data found. Please remove before committing.")
    return code_changes

def secure_api_key_storage(api_key):
    key = Fernet.generate_key()
    fernet = Fernet(key)
    encrypted_key = fernet.encrypt(api_key.encode())
    return encrypted_key, key

class KeyManagementSystem:
    def __init__(self, master_key):
        self.master_key = master_key
        self.kdf = PBKDF2HMAC(
            algorithm=hashes.SHA256(),
            length=32,
            salt=os.urandom(16),
            iterations=100000,
            backend=default_backend()
        )

    def generate_key(self, key_id):
        key = base64.urlsafe_b64encode(self.kdf.derive(key_id.encode()))
        return Fernet(key)

    def encrypt(self, key_id, data):
        f = self.generate_key(key_id)
        return f.encrypt(data.encode())

    def decrypt(self, key_id, encrypted_data):
        try:
            f = self.generate_key(key_id)
            return f.decrypt(encrypted_data).decode()
        except Exception as e:
            raise ValueError(f"Decryption failed: {str(e)}")

def load_environment_variables():
    load_dotenv(raise_if_not_found=True)

def get_environment_variable(key, default=None):
    value = os.getenv(key, default)
    if value is None:
        raise ValueError(f"Environment variable '{key}' not found and no default provided.")
    return value

def set_environment_variable(key, value):
    if not re.match(r'^[a-zA-Z_][a-zA-Z0-9_]*$', key):
        raise ValueError("Invalid environment variable name.")
    os.environ[key] = str(value)

if __name__ == "__main__":
    # Use environment variables
    configured_env = use_environment_variables("my_sensitive_data")
    print("Sensitive data stored in environment variable:", configured_env['SENSITIVE_DATA'])

    # Prevent secret commits
    try:
        changes = "Some code changes API_KEY='secret123'"
        sanitized_changes = prevent_secret_commits(changes)
    except ValueError as e:
        print("Commit prevented:", str(e))

    # Secure API key storage
    api_key = "my_api_key"
    stored_key, encryption_key = secure_api_key_storage(api_key)
    print("Encrypted API key:", stored_key)

    # Key Management System
    master_key = os.urandom(32)
    kms = KeyManagementSystem(master_key)

    encrypted_api_key = kms.encrypt("api_key_1", api_key)
    print("KMS Encrypted API key:", encrypted_api_key)

    decrypted_api_key = kms.decrypt("api_key_1", encrypted_api_key)
    print("KMS Decrypted API key:", decrypted_api_key)

    # Environment Variable Management
    load_environment_variables()

    api_key = get_environment_variable("API_KEY", "default_api_key")
    print("API Key:", api_key)

    set_environment_variable("NEW_SECRET", "very_secret_value")
    print("New secret:", get_environment_variable("NEW_SECRET"))