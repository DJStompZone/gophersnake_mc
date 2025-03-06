#!/usr/bin/env python3

import requests
import json
import os
import time
import sys
import atexit
from pprint import pprint
from datetime import datetime, timedelta

# Try to import MSAL or provide guidance if it's not installed
try:
    import msal
except ImportError:
    print("ERROR: The 'msal' package is required but not installed.", file=sys.stderr)
    print("Please install it using: pip install msal", file=sys.stderr)
    print("Then run this script again.", file=sys.stderr)
    sys.exit(1)

# Output diagnostic info at startup to stderr
debug_print = lambda *args, **kwargs: print(*args, file=sys.stderr, **kwargs)
debug_print(f"XBL3 Token Script starting (Python {sys.version})")
debug_print(f"Current working directory: {os.getcwd()}")
debug_print(f"Script path: {os.path.abspath(__file__)}")

# Redirect all print statements to stderr except for the final token
# This ensures that only the token is captured by the Go program
def debug_print(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)

# Use MSAL's SerializableTokenCache instead of custom CacheManager
# Determine cache location based on platform and user rights
def get_cache_location():
    # Try using standard location first
    try:
        cache_dir = os.getenv("XDG_RUNTIME_DIR", None)
        if cache_dir and os.path.exists(cache_dir) and os.access(cache_dir, os.W_OK):
            return os.path.join(cache_dir, "xbl3_token_cache.bin")
    except Exception as e:
        print(f"{e.__class__.__name__}: {e}")
    
    # Try app directory if available
    try:
        app_dir = os.path.dirname(os.path.abspath(__file__))
        if os.access(app_dir, os.W_OK):
            return os.path.join(app_dir, "xbl3_token_cache.bin")
    except Exception as e:
        print(f"{e.__class__.__name__}: {e}")
    
    # Fall back to temp directory
    try:
        import tempfile
        temp_dir = tempfile.gettempdir()
        if os.access(temp_dir, os.W_OK):
            return os.path.join(temp_dir, "xbl3_token_cache.bin")
    except Exception as e:
        print(f"{e.__class__.__name__}: {e}")
    
    # If all else fails, use in-memory cache only (no persistence)
    return None

# Determine cache file location
cache_filename = get_cache_location()
debug_print(f"Token cache location: {cache_filename or 'In-memory only (no persistence)'}")

class MsaTokenManager:
    """Handles Microsoft authentication and token management using MSAL"""

    # Use the same CLIENT_ID as in xbl3.py to ensure consistent authentication behavior
    CLIENT_ID = "93819583-abf7-4a5e-8b53-9526cf7e7ba9"
    AUTHORITY = "https://login.microsoftonline.com/consumers/"
    SCOPES = ["Xboxlive.signin", "Xboxlive.offline_access"]

    def __init__(self):
        # Initialize the serializable token cache
        self.token_cache = msal.SerializableTokenCache()
        
        # Load the cache from file if it exists
        if cache_filename and os.path.exists(cache_filename):
            try:
                with open(cache_filename, "r") as f:
                    self.token_cache.deserialize(f.read())
                debug_print("Loaded token cache from file")
            except Exception as e:
                debug_print(f"Failed to load token cache: {e}")
                debug_print("Starting with fresh token cache")
        else:
            # create a new cache file
            with open(cache_filename or "xbl3_token_cache.bin", "w") as f:
                f.write(self.token_cache.serialize())
            debug_print("Created new token cache file")
        
        # Register to save cache on exit
        atexit.register(self._save_cache)
        
        # Initialize MSAL app with our token cache
        self.msal_app = msal.PublicClientApplication(
            self.CLIENT_ID, 
            authority=self.AUTHORITY,
            token_cache=self.token_cache
        )

    def _save_cache(self):
        """Save the token cache to disk if it has changed"""
        if self.token_cache.has_state_changed and cache_filename:
            try:
                os.makedirs(os.path.dirname(cache_filename), exist_ok=True)
                with open(cache_filename, "w") as f:
                    f.write(self.token_cache.serialize())
                debug_print("Token cache saved to file")
            except Exception as e:
                debug_print(f"Failed to save token cache: {e}")
                debug_print("Continuing with in-memory cache only")

    def get_access_token(self):
        """Returns a valid access token, refreshing if necessary"""
        # Look for token in cache first
        accounts = self.msal_app.get_accounts()
        if accounts:
            # Get token from cache if available
            result = self.msal_app.acquire_token_silent(
                self.SCOPES, account=accounts[0]
            )
            if result and "access_token" in result:
                debug_print("Retrieved token from cache")
                return {"valid": True, "token": result["access_token"]}
        
        # If no valid token in cache, try to refresh or prompt for new auth
        return self.refresh_tokens()

    def get_refresh_token(self):
        """Gets the refresh token if available"""
        accounts = self.msal_app.get_accounts()
        if not accounts:
            return None
        return accounts[0].get("refresh_token")

    def refresh_tokens(self):
        """Attempt to refresh the token or initiate a new auth flow"""
        # Check if we have accounts that can be used for silent token acquisition
        accounts = self.msal_app.get_accounts()
        if accounts:
            debug_print(f"Found {len(accounts)} account(s) in cache, attempting silent token refresh")
            result = self.msal_app.acquire_token_silent(
                self.SCOPES, account=accounts[0]
            )
            if result and "access_token" in result:
                debug_print("Token refreshed successfully")
                return {"valid": True, "token": result["access_token"]}
            
            debug_print("Silent token refresh failed, initiating new auth flow")
        
        # If we couldn't refresh silently, initiate device code flow
        return self.auth_device_code()

    def auth_device_code(self):
        """Authenticate using the device code flow"""
        debug_print("Starting device code authentication flow")
        flow = self.msal_app.initiate_device_flow(scopes=self.SCOPES)
        
        if "user_code" not in flow:
            debug_print(f"Failed to initiate device flow: {flow.get('error_description', 'Unknown error')}")
            return {"valid": False, "error": "Failed to initiate device flow"}
        
        # Print instructions for user authentication
        debug_print("\n" + "*" * 70)
        debug_print(flow["message"])
        debug_print("*" * 70 + "\n")
        
        # Wait for the user to complete the authentication
        result = self.msal_app.acquire_token_by_device_flow(flow)
        
        if "access_token" not in result:
            debug_print(f"Authentication failed: {result.get('error_description', 'Unknown error')}")
            return {"valid": False, "error": result.get("error_description", "Unknown error")}
        
        debug_print("Authentication successful")
        return {"valid": True, "token": result["access_token"]}

class XboxLiveAuth:
    """Handles Xbox Live and XSTS authentication"""

    XBL_AUTH_URL = "https://user.auth.xboxlive.com/user/authenticate"
    XSTS_AUTH_URL = "https://xsts.auth.xboxlive.com/xsts/authorize"

    def __init__(self, msa_manager):
        self.msa_manager = msa_manager

    def get_xbl3_token(self):
        """Main entry point to get XBL3.0 token"""
        return self.authenticate()

    def authenticate(self):
        """Perform full authentication flow from MSA to XBL3.0 token"""
        # Step 1: Get Microsoft access token
        msa_token_result = self.msa_manager.get_access_token()
        if not msa_token_result["valid"]:
            debug_print(f"Failed to get MSA token: {msa_token_result.get('error', 'Unknown error')}")
            return None

        msa_token = msa_token_result["token"]
        debug_print("Successfully obtained MSA token")

        # Step 2: Get Xbox Live token
        xbl_token = self.get_xbl_token(msa_token)
        if not xbl_token:
            debug_print("Failed to get XBL token")
            return None
        debug_print("Successfully obtained XBL token")

        # Step 3: Get XSTS token
        xsts_token = self.get_xsts_token(xbl_token)
        if not xsts_token:
            debug_print("Failed to get XSTS token")
            return None
        debug_print("Successfully obtained XSTS token")

        # Step 4: Format the final XBL3.0 token
        uhs = xsts_token["DisplayClaims"]["xui"][0]["uhs"]
        token = xsts_token["Token"]
        xbl3_token = f"XBL3.0 x={uhs};{token}"
        
        debug_print(f"XBL3.0 token generated successfully (UHS: {uhs})")
        return xbl3_token

    def get_xbl_token(self, msa_token):
        """Exchange MSA token for Xbox Live token"""
        data = None
        try:
            payload = {
                "Properties": {
                    "AuthMethod": "RPS",
                    "SiteName": "user.auth.xboxlive.com",
                    "RpsTicket": f"d={msa_token}"
                },
                "RelyingParty": "http://auth.xboxlive.com",
                "TokenType": "JWT"
            }
            
            debug_print(f"Sending XBL token request with payload: {payload}")
            
            response = requests.post(
                self.XBL_AUTH_URL,
                json=payload,
                headers={"Content-Type": "application/json"}
            )
            
            response.raise_for_status()
            data = response.json()
            debug_print(f"XBL token response format: {list(data.keys())}")
            
            return data
        except requests.RequestException as e:
            debug_print(f"Request failed during XBL token acquisition: {e}")
            return None
        except KeyError as e:
            debug_print(f"Unexpected response format from XBL auth: {e}")
            debug_print(f"Response data: {data}")
            return None
        except Exception as e:
            debug_print(f"Unexpected error during XBL token acquisition: {e}")
            debug_print(f"Response data: {data}")
            return None

    def get_xsts_token(self, xbl_token):
        """Exchange Xbox Live token for XSTS token"""
        data = None
        try:
            payload = {
                "Properties": {
                    "SandboxId": "RETAIL",
                    "UserTokens": [xbl_token["Token"]]
                },
                "RelyingParty": "rp://api.minecraftservices.com/",
                "TokenType": "JWT"
            }
            
            debug_print(f"Sending XSTS token request with UserTokens length: {len(payload['Properties']['UserTokens'])}")
            
            response = requests.post(
                self.XSTS_AUTH_URL,
                json=payload,
                headers={"Content-Type": "application/json"}
            )
            
            response.raise_for_status()
            data = response.json()
            debug_print(f"XSTS token response format: {list(data.keys())}")
            
            return data
        except requests.RequestException as e:
            debug_print(f"Request failed during XSTS token acquisition: {e}")
            return None
        except KeyError as e:
            debug_print(f"Unexpected response format from XSTS auth: {e}")
            debug_print(f"Response data: {data}")
            return None
        except Exception as e:
            debug_print(f"Unexpected error during XSTS token acquisition: {e}")
            debug_print(f"Response data: {data}")
            return None

# Main execution
if __name__ == "__main__":
    try:
        msa_manager = MsaTokenManager()
        xbox_auth = XboxLiveAuth(msa_manager)
        
        token = xbox_auth.get_xbl3_token()
        if token:
            # Print only the token to stdout for the Go program to capture
            print(token)
            sys.exit(0)
        else:
            debug_print("Failed to get XBL3.0 token")
            sys.exit(1)
    except Exception as e:
        debug_print(f"Unhandled exception: {e}")
        sys.exit(1)