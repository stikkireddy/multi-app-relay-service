import time

import streamlit as st
import streamlit_shadcn_ui as ui
from streamlit_server_state import server_state_lock, server_state
import requests
import os

from dataclasses import dataclass, field
from streamlit.web.server.websocket_headers import _get_websocket_headers

PORT = os.environ.get("DATABRICKS_APP_PORT", "8000")
MANAGEMENT_API_URL = f"http://0.0.0.0:{PORT}"
APPS_API_URL = f"{MANAGEMENT_API_URL}/apps"

def generate_forwarded_url():
    headers = _get_websocket_headers()
    if headers is None:
        return ""
    forwarded_host = headers.get("X-Forwarded-Host")
    # always http for now
    forwarded_proto = "http"
    if forwarded_host is None or forwarded_proto is None:
        return ""
    forwarded = f"{forwarded_proto}://{forwarded_host}"
    return forwarded

@dataclass
class AppTile:
    app_name: str
    title: str
    description: str
    launch_url: str
    logs_url: str
    start_url: str
    stop_url: str
    tags: list[str] = field(default_factory=list)
    logo_url: str = "https://via.placeholder.com/400"
    status: str = "terminated"

    @property
    def key(self):
        return self.title.lower().replace(" ", "_")

    @property
    def app_status_key(self):
        return self.key + "_app_status"

    def refresh_status(self):
        resp = requests.get(APPS_API_URL)
        resp.raise_for_status()
        result = resp.json()
        status_map = result.get("statuses", {})
        self.status = status_map.get(self.app_name, "terminated")

    def start_app(self):
        resp = requests.post(self.start_url)
        resp.raise_for_status()
        self.refresh_status()

    def stop_app(self):
        resp = requests.post(self.stop_url)
        resp.raise_for_status()
        self.refresh_status()

def get_tiles():
    resp = requests.get(APPS_API_URL)
    resp.raise_for_status()
    result = resp.json()
    apps = result.get("cfg", {}).get("apps", [])
    status_map = result.get("statuses", {})
    tiles = []
    for app in apps:
        meta = app.get("meta", {})
        route = app.get("routePath", "/")
        tiles.append(
            AppTile(
                app_name=app["name"],
                title=meta["title"],
                description=meta["description"],
                logs_url=generate_forwarded_url() + "/relay/" + route.lstrip("/").rstrip("/") + "/_logz",
                launch_url=generate_forwarded_url() + "/relay/" + route.lstrip("/"),
                start_url=MANAGEMENT_API_URL + "/" + route.lstrip("/").rstrip("/") + "/start",
                stop_url=MANAGEMENT_API_URL + "/" + route.lstrip("/").rstrip("/") + "/kill",
                tags=meta.get("tags", []),
                logo_url=meta.get("logo_url", "https://via.placeholder.com/400"),
                status=status_map.get(app["name"], "terminated")
            )
        )
    return tiles


tile_data = get_tiles()


def normalize_title(title):
    return title.lower().replace(" ", "_")


def get_app_status_key(_tile: AppTile):
    return _tile.app_status_key


for tile in tile_data:
    app_status_key = get_app_status_key(tile)
    with (server_state_lock[app_status_key]):  # Lock the "count" state for thread-safety
        server_state[app_status_key] = tile.status


def start_app_api_call(tile: AppTile):

    tile.start_app()

    for i in range(10):
        tile.refresh_status()
        if tile.status == "running" or tile.status == "terminated":
            break
        time.sleep(1)

    with server_state_lock[get_app_status_key(tile)]:  # Lock the "count" state for thread-safety
        server_state[get_app_status_key(tile)] = tile.status  # Update the app state to running
    # progress_bar.empty()  # Remove the progress indicator
    return True  # Return True when finished


def stop_app_api_call(tile: AppTile):
    tile.stop_app()
    for i in range(10):
        tile.refresh_status()
        if tile.status == "terminated":
            break
        time.sleep(1)

    with server_state_lock[get_app_status_key(tile)]:  # Lock the "count" state for thread-safety
        server_state[get_app_status_key(tile)] = tile.status  # Update the app state to running
    # progress_bar.empty()  # Remove the progress indicator
    return True  # Return True when finished


# Function to create a tile
def create_tile(tile: AppTile):
    st.image(tile.logo_url)
    st.subheader(tile.title)
    st.write(tile.description)
    ui.badges(badge_list=[(tag, "secondary") for tag in tile.tags], class_name="flex gap-2",
              key=tile.title + "badges")

    app_status_key = get_app_status_key(tile)
    with server_state_lock[app_status_key]:
        app_status = server_state[app_status_key]

    st.write(f"App Status: {app_status}")

    col1, col2 = st.columns([1, 4], gap="small")

    if app_status == "terminated":
        if col1.button("Run", key=tile.key + "_run"):
            with col2:
                with st.spinner("Starting app..."):
                    if start_app_api_call(tile):
                        col1.button("Stop", key=tile.key + "_stop", disabled=False)
                    else:
                        col1.button("Run", key=tile.key + "_run", disabled=True)
    else:
        if col1.button("Stop", key=tile.key + "_stop"):
            with col2:
                with st.spinner("Stopping app..."):
                    if stop_app_api_call(tile):
                        col1.button("Run", key=tile.key + "_run", disabled=False)

    # if app_status == "running":
    col2.markdown(f"[Launch App]({tile.launch_url}){'&nbsp;' * 8}[App Logs]({tile.logs_url})",
                  unsafe_allow_html=True)


def filter_tiles(tiles, query):
    if not query:
        return tiles
    keywords = query.lower().split()
    return [
        tile for tile in tiles
        if any(keyword in tile.title.lower() or keyword in tile.description.lower()
               or any(keyword in tag.lower() for tag in tile.tags)
               for keyword in keywords)
    ]


# App layout
def main():
    st.set_page_config(layout="wide")
    st.title("App Gallery")
    search_query = st.text_input("Search", key="search")

    st.markdown("---")

    # Layout for tiles
    filtered_tiles = filter_tiles(tile_data, search_query)

    # Layout for tiles
    if filtered_tiles:
        cols = st.columns(min(len(filtered_tiles), 3))
        for i, data in enumerate(filtered_tiles):
            with cols[i % 3]:
                create_tile(data)
    else:
        st.write("No matching tiles found.")


if __name__ == "__main__":
    main()
