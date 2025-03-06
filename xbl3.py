import requests
import msal
import json
import os
import time
from datetime import datetime, timedelta

class CacheManager:
    """Handles caching of access tokens to prevent redundant authentication"""
    
    CACHE_FILE = "xbl_token_cache.json"

    def get_cached(self):
        """Retrieve cached tokens from file"""
        if os.path.exists(self.CACHE_FILE):
            with open(self.CACHE_FILE, "r") as f:
                return json.load(f)
        return {}

    def set_cached_partial(self, data):
        """Update and save partial token cache"""
        cached_data = self.get_cached()
        cached_data.update(data)
        with open(self.CACHE_FILE, "w") as f:
            json.dump(cached_data, f)

class MsaTokenManager:
    """Handles Microsoft authentication and token management using MSAL"""

    CLIENT_ID = "93819583-abf7-4a5e-8b53-9526cf7e7ba9"
    AUTHORITY = "https://login.microsoftonline.com/consumers/"
    SCOPES = ["Xboxlive.signin", "Xboxlive.offline_access"]

    def __init__(self):
        self.cache = CacheManager()
        self.msal_app = msal.PublicClientApplication(
            self.CLIENT_ID, authority=self.AUTHORITY
        )

    def get_access_token(self):
        """Returns a valid access token, refreshing if necessary"""
        tokens = self.cache.get_cached().get("AccessToken", {})
        if not tokens:
            return None

        account = next((t for t in tokens.values() if t["client_id"] == self.CLIENT_ID), None)
        if not account:
            return None

        expires_at = datetime.utcfromtimestamp(account["expires_on"])
        if expires_at > datetime.utcnow():
            return {"valid": True, "token": account["secret"]}

        return self.refresh_tokens()

    def get_refresh_token(self):
        """Retrieves a cached refresh token if available"""
        tokens = self.cache.get_cached().get("RefreshToken", {})
        return next((t["secret"] for t in tokens.values() if t["client_id"] == self.CLIENT_ID), None)

    def refresh_tokens(self):
        """Refreshes the access token using a refresh token"""
        rtoken = self.get_refresh_token()
        if not rtoken:
            raise ValueError("Cannot refresh without a refresh token")

        refresh_request = {
            "refresh_token": rtoken,
            "scopes": self.SCOPES
        }

        try:
            result = self.msal_app.acquire_token_by_refresh_token(**refresh_request)
            if "access_token" in result:
                self.cache.set_cached_partial({"AccessToken": {"secret": result["access_token"], "expires_on": time.time() + 3600}})
                return {"valid": True, "token": result["access_token"]}
        except Exception as e:
            print(f"Error refreshing token: {e}")
            return None

    def auth_device_code(self):
        """Authenticate using device code flow"""
        print(f"Authenticating using device code flow... {self.SCOPES=}")
        device_code_flow = self.msal_app.initiate_device_flow(scopes=self.SCOPES)
        print(f"Device code flow: {device_code_flow}")
        if "message" not in device_code_flow:
            raise ValueError("Failed to initiate device code authentication")

        print(device_code_flow["message"])

        result = self.msal_app.acquire_token_by_device_flow(device_code_flow)
        if "access_token" not in result:
            raise ValueError(f"Error acquiring token: {result.get('error_description', 'Unknown error')}")

        self.cache.set_cached_partial({
            "AccessToken": {"secret": result["access_token"], "expires_on": time.time() + 3600},
            "RefreshToken": {"secret": result["refresh_token"]}
        })
        return result["access_token"]

class XboxLiveAuth:
    """Handles Xbox Live and XSTS authentication"""

    XBL_AUTH_URL = "https://user.auth.xboxlive.com/user/authenticate"
    XSTS_AUTH_URL = "https://xsts.auth.xboxlive.com/xsts/authorize"

    def __init__(self, msa_manager):
        self.msa_manager = msa_manager

    def get_xbl3_token(self):
        """Fetches or refreshes an XBL3.0 token"""
        token_data = self.msa_manager.cache.get_cached().get("XBL3Token", {})
        if token_data and datetime.utcfromtimestamp(token_data["expires_on"]) > datetime.utcnow():
            return token_data["token"]

        print("Fetching new XBL3.0 token...")
        return self.authenticate()

    def authenticate(self):
        """Performs full authentication flow to retrieve XBL3.0 token"""
        msa_token = self.msa_manager.get_access_token()
        if not msa_token:
            msa_token = self.msa_manager.auth_device_code()

        xbl_token, uhs = self.get_xbl_token(msa_token["token"])
        xsts_token = self.get_xsts_token(xbl_token)

        xbl3_token = f"XBL3.0 x={uhs};{xsts_token}"
        self.msa_manager.cache.set_cached_partial({"XBL3Token": {"token": xbl3_token, "expires_on": time.time() + 82800}})  # 23 hrs validity
        return xbl3_token

    def get_xbl_token(self, msa_token):
        """Exchanges an MSA token for an Xbox Live token"""
        payload = {
            "Properties": {
                "AuthMethod": "RPS",
                "SiteName": "user.auth.xboxlive.com",
                "RpsTicket": f"d={msa_token}"
            },
            "RelyingParty": "http://auth.xboxlive.com",
            "TokenType": "JWT"
        }
        response = requests.post(self.XBL_AUTH_URL, json=payload).json()
        return response["Token"], response["DisplayClaims"]["xui"][0]["uhs"]

    def get_xsts_token(self, xbl_token):
        """Exchanges an Xbox Live token for an XSTS token"""
        payload = {
            "Properties": {
                "SandboxId": "RETAIL",
                "UserTokens": [xbl_token]
            },
            "RelyingParty": "http://xboxlive.com",
            "TokenType": "JWT"
        }
        response = requests.post(self.XSTS_AUTH_URL, json=payload).json()
        return response["Token"]