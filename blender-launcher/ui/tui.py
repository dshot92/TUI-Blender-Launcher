import signal
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple, Union, cast
import time

from rich.console import Console
from rich.table import Table
from rich.text import Text
from rich.panel import Panel

from ..api.builder_api import fetch_builds
from ..config.app_config import AppConfig
from ..models.build_info import BlenderBuild, LocalBuildInfo
from ..utils.input import (
    get_keypress,
    prompt_input,
    KEY_UP,
    KEY_DOWN,
    KEY_LEFT,
    KEY_RIGHT,
    KEY_ENTER,
    KEY_ENTER_ALT,
    KEY_SPACE,
)
from ..utils.local_builds import get_local_builds, delete_local_build


@dataclass
class UIState:
    """Holds the state of the TUI."""

    builds: List[BlenderBuild] = field(default_factory=list)
    selected_builds: Set[str] = field(
        default_factory=set
    )  # Now stores versions instead of indices
    current_page: str = "builds"  # "builds" or "settings"
    cursor_position: int = 0  # Current cursor position for navigation
    settings_cursor: int = 0  # Cursor position in settings page
    needs_refresh: bool = False  # Flag to indicate if a refresh is needed
    has_fetched: bool = False  # Flag to track if we've fetched builds from online
    local_builds: Dict[str, LocalBuildInfo] = field(default_factory=dict)
    # Sort configuration
    sort_column: int = (
        2  # Default sort on Version column (index 2 after removing cursor column)
    )
    sort_reverse: bool = True  # Sort descending by default
    # Download progress tracking
    download_progress: Dict[str, float] = field(default_factory=dict)
    download_speed: Dict[str, str] = field(default_factory=dict)


class BlenderTUI:
    """Text User Interface for Blender build management."""

    def __init__(self):
        """Initialize the BlenderTUI."""
        # Get fixed download path
        self.download_path = AppConfig.DOWNLOAD_PATH

        # Create console
        self.console = Console()

        # Track terminal size
        self.terminal_width, self.terminal_height = self.console.size

        # Add signal handler for terminal resize
        signal.signal(signal.SIGWINCH, self._handle_resize)

        # Create UI state
        self.state = UIState()

        # Get local builds to start
        self.state.local_builds = get_local_builds()

        # Initialize page handlers
        self.page_handlers = {
            "builds": self._handle_builds_page_input,
            "settings": self._handle_settings_page_input,
        }

    def _handle_resize(self, *args) -> None:
        """Handle terminal resize events by setting a flag to refresh the UI."""
        # Update terminal size immediately
        self.terminal_width, self.terminal_height = self.console.size
        self.state.needs_refresh = True

    def clear_screen(self, full_clear=True) -> None:
        """Clear the screen completely or just position cursor at home position.

        Args:
            full_clear: If True, completely clear the screen. If False, only position cursor.
        """
        if full_clear:
            # Use ANSI sequence to clear the entire screen
            print("\033[2J", end="", flush=True)

        # Move cursor to home position
        print("\033[H", end="", flush=True)

        # Hide the cursor to prevent it from flashing
        print("\033[?25l", end="", flush=True)

    def print_navigation_bar(self) -> None:
        """Print a navigation bar with keybindings at the bottom of the screen."""
        if self.state.current_page == "builds":
            # Create a more visually appealing separator
            self.console.print("━" * self.console.width, style="dim")

            # Group commands by functionality with better spacing and styling
            actions = "[bold yellow]Space[/bold yellow]:Select  [bold green]Enter[/bold green]:Launch  [bold cyan]D[/bold cyan]:Download  [bold red]X[/bold red]:Delete"
            system = "[bold magenta]F[/bold magenta]:Fetch  [bold magenta]R[/bold magenta]:Reverse  [bold magenta]S[/bold magenta]:Settings  [bold magenta]Q[/bold magenta]:Quit"

            # Format in a visually balanced way
            self.console.print(f"{actions}  │  {system}", highlight=False)
        else:  # settings page
            # Create a more visually appealing separator for settings
            self.console.print("━" * self.console.width, style="dim")

            # Group commands for settings page
            actions = "[bold green]Enter[/bold green]:Edit Setting"
            system = "[bold magenta]S[/bold magenta]:Back to Builds  [bold magenta]Q[/bold magenta]:Quit"

            # Format in a visually balanced way
            self.console.print(f"{actions}  │  {system}", highlight=False)

    def print_build_table(self) -> None:
        """Print a table of available Blender builds."""
        self.state.local_builds = get_local_builds()

        # Update terminal size before rendering
        self.terminal_width, self.terminal_height = self.console.size

        # Base column names for sorting indication
        base_columns = [
            "",  # Selection column
            "Version",
            "Status",
            "Branch",
            "Type",
            "Hash",
            "Size",
        ]

        # Apply responsive column hiding based on terminal width
        column_names = self._get_responsive_columns(base_columns)

        # Use "Speed" instead of "Build Date" when downloads are active
        if self.state.download_progress:
            column_names.append("Speed")
        else:
            column_names.append("Build Date")

        table = self._create_table(column_names)

        # Get the combined list of builds
        combined_build_list = self._get_combined_build_list()

        if not combined_build_list:
            self.console.print(
                "No builds found. Press F to fetch online builds.",
                style="bold yellow",
            )
            return

        # Add all builds to the table
        for i, (build_type, original_idx, version) in enumerate(combined_build_list):
            # Check if this version is selected using version instead of index
            selected = "[X]" if version in self.state.selected_builds else "[ ]"
            row_style = "reverse bold" if i == self.state.cursor_position else ""

            if build_type == "local":
                # It's a local build
                build_info = self.state.local_builds[version]
                type_text = (
                    self._get_style_for_risk_id(build_info.risk_id)
                    if build_info.risk_id
                    else ""
                )
                build_date = self._format_build_date(build_info)

                # Default style is yellow for local builds
                version_style = "yellow"
                version_prefix = "■ "
                status_text = Text("Local", style="yellow bold")

                # Hash information (might be None for local builds)
                hash_text = getattr(build_info, "hash", "") or ""

                # Get size information
                size_text = ""
                if build_info.size_mb is not None:
                    size_text = f"{build_info.size_mb:.1f}MB"

                # Check if this build is also in online builds
                if self.state.has_fetched:
                    for online_build in self.state.builds:
                        if online_build.version == version:
                            # This local build also exists online, check if update available
                            if (
                                build_info.time
                                and online_build.build_time
                                and build_info.time != online_build.build_time
                            ):
                                type_text = Text("update available", style="green bold")
                                version_style = (
                                    "green"  # Change to green for update available
                                )
                                status_text = Text("Update", style="green bold")
                                # Show online hash if an update is available
                                hash_text = online_build.hash or ""

                # Prepare row data
                row_data = [
                    selected,
                    Text(f"{version_prefix}Blender {version}", style=version_style),
                    status_text,
                ]

                # Only add Branch column if it's in our responsive columns
                if "Branch" in column_names:
                    row_data.append(build_info.branch or "")

                row_data.append(type_text)

                # Only add Hash column if it's in our responsive columns
                if "Hash" in column_names:
                    row_data.append(hash_text)

                # Only add Size column if it's in our responsive columns
                if "Size" in column_names:
                    row_data.append(size_text)

                row_data.append(build_date)

                table.add_row(*row_data, style=row_style)
            else:
                # It's an online build
                online_build = None
                for build in self.state.builds:
                    if build.version == version:
                        online_build = build
                        break

                if not online_build:
                    continue  # Skip if build not found

                type_text = self._get_style_for_risk_id(online_build.risk_id)
                status_text = Text("Online", style="blue bold")
                hash_text = online_build.hash or ""  # Add hash

                # Check if this build is being downloaded
                is_downloading = version in self.state.download_progress
                download_progress = self.state.download_progress.get(version, 0)
                download_speed = self.state.download_speed.get(version, "")

                if is_downloading:
                    # For downloading builds, show progress in the size column and speed in the date column
                    size_text = f"{download_progress:.0f}%"
                    date_text = download_speed
                    type_text = Text("downloading", style="green bold")
                    version_col = Text(f"↓ Blender {version}", style="green bold")
                    status_text = Text("Downloading", style="green bold")
                else:
                    size_text = f"{online_build.size_mb:.1f}MB"
                    date_text = online_build.mtime_formatted
                    version_col = Text(f"  Blender {version}", style="default")

                # Prepare row data
                row_data = [
                    selected,
                    version_col,
                    status_text,
                ]

                # Only add Branch column if it's in our responsive columns
                if "Branch" in column_names:
                    row_data.append(online_build.branch)

                row_data.append(type_text)

                # Only add Hash column if it's in our responsive columns
                if "Hash" in column_names:
                    row_data.append(hash_text)

                # Only add Size column if it's in our responsive columns
                if "Size" in column_names:
                    row_data.append(size_text)

                row_data.append(date_text)

                table.add_row(*row_data, style=row_style)

        # Print the table
        self.console.print(table)

        # Display legend at the top of the page if we have online builds
        if self.state.has_fetched and self.state.builds:
            legend = Text()
            legend.append("■ Current Local Version", style="yellow")
            legend.append("   ")
            legend.append("■ Update Available", style="green")
            self.console.print(legend)

    def _get_responsive_columns(self, base_columns):
        """Determine which columns to show based on terminal width.

        Args:
            base_columns: The base set of column names

        Returns:
            List of column names adjusted for the current terminal width
        """
        # Make a copy of the base columns
        columns = base_columns.copy()

        # Minimum width required for a comfortably usable display
        # (selection, version, status, type, and date)
        min_required_width = 80

        # Remove columns in order of preference as terminal width decreases
        # 1. First remove Branch (lowest priority to keep)
        if self.terminal_width < 115:
            if "Branch" in columns:
                columns.remove("Branch")

        # 2. Then remove Hash
        if self.terminal_width < 100:
            if "Hash" in columns:
                columns.remove("Hash")

        # 3. Finally remove Size
        if self.terminal_width < min_required_width:
            if "Size" in columns:
                columns.remove("Size")

        return columns

    def _clear_remaining_lines(self) -> None:
        """Clear any remaining content lines after table rendering.
        This prevents ghost content from previous renders persisting on screen.
        """
        # Use ANSI escape sequence to clear from cursor to end of screen
        print("\033[J", end="", flush=True)

    def _create_table(self, column_names: List[str]) -> Table:
        """Create and configure the table with proper columns.

        Args:
            column_names: List of column header names

        Returns:
            Configured table object
        """
        # Use a standard box style from Rich
        from rich import box
        from rich.text import Text

        table = Table(show_header=True, expand=True, box=box.SIMPLE_HEAVY)

        # Map of column indices in the full columns list to indices in the responsive columns list
        column_index_map = {
            0: 0,  # Selection column is always at index 0
            1: 1,  # Version column is always at index 1
            2: 2,  # Status column is always at index 2
        }

        # Track current index in column_names
        current_idx = 3

        # Branch column (index 3 in full list)
        if "Branch" in column_names:
            column_index_map[3] = current_idx
            current_idx += 1

        # Type column
        column_index_map[4] = current_idx
        current_idx += 1

        # Hash column (index 5 in full list)
        if "Hash" in column_names:
            column_index_map[5] = current_idx
            current_idx += 1

        # Size column (index 6 in full list)
        if "Size" in column_names:
            column_index_map[6] = current_idx
            current_idx += 1

        # Build Date column (index 7 in full list)
        column_index_map[7] = current_idx

        # Adjust sort column if necessary
        adjusted_sort_column = self.state.sort_column
        if self.state.sort_column in column_index_map:
            adjusted_sort_column = column_index_map[self.state.sort_column]
        elif self.state.sort_column > 7:
            # If sort column is beyond our mapping (like 8 for Date), just use the last column
            adjusted_sort_column = len(column_names) - 1

        for i, col_name in enumerate(column_names):
            # Add sort indicator to column header
            if i == adjusted_sort_column - 1:  # Adjust index for removed column
                sort_indicator = "↑" if not self.state.sort_reverse else "↓"
                header_text = Text(f"{col_name} {sort_indicator}", style="reverse bold")
            else:
                header_text = col_name

            # Add columns with appropriate configuration
            if col_name == "":  # Selection column
                table.add_column(
                    header_text, justify="center", width=2
                )  # Will fit [X] or [ ]
            elif col_name == "Version":
                table.add_column(header_text, justify="left")  # Variable width
            elif col_name == "Status":
                table.add_column(header_text, justify="left", width=6)  # Status column
            elif col_name == "Branch":
                table.add_column(header_text, justify="center")  # Will fit branch names
            elif col_name == "Type":
                table.add_column(header_text, justify="center")  # Will fit risk types
            elif col_name == "Hash":
                table.add_column(
                    header_text, justify="center", width=12
                )  # Will fit hash values
            elif col_name == "Size":
                table.add_column(header_text, justify="center")  # Will fit size values
            elif col_name == "Build Date" or col_name == "Speed":
                table.add_column(header_text, justify="center")  # Will fit dates

        return table

    def _add_online_builds_to_table(self, table: Table) -> None:
        """Add online builds to the table.

        Args:
            table: Table to add rows to
        """
        sorted_builds = self.sort_builds(self.state.builds)

        for i, build in enumerate(sorted_builds):
            selected = "[X]" if i in self.state.selected_builds else "[ ]"
            type_text = self._get_style_for_risk_id(build.risk_id)
            hash_text = build.hash or ""  # Add hash column

            # Check if this build is being downloaded
            is_downloading = build.version in self.state.download_progress
            download_progress = self.state.download_progress.get(build.version, 0)
            download_speed = self.state.download_speed.get(build.version, "")

            # Determine version style based on local builds or download status
            if is_downloading:
                # For downloading builds, show progress in the size column and speed in the date column
                size_text = f"{download_progress:.0f}%"
                date_text = str(download_speed)  # Ensure it's a string

                # Set type column to show "downloading"
                type_text = Text("downloading", style="green bold")

                # Set row style for downloading build without red background
                row_style = "reverse bold" if i == self.state.cursor_position else ""

                # Set version with indicator
                version_col = Text(f"↓ Blender {build.version}", style="green bold")
                status_text = Text("Downloading", style="green bold")

            elif build.version in self.state.local_builds:
                local_info = self.state.local_builds[build.version]
                style = (
                    "yellow"
                    if local_info.time == build.build_time or not local_info.time
                    else "green"
                )
                version_col = Text(f"■ Blender {build.version}", style=style)
                row_style = "reverse bold" if i == self.state.cursor_position else ""
                size_text = f"{build.size_mb:.1f}MB"
                date_text = str(build.mtime_formatted)
                status_text = Text("Online", style="blue bold")  # Status
            else:
                version_col = Text(f"  Blender {build.version}")
                row_style = "reverse bold" if i == self.state.cursor_position else ""
                size_text = f"{build.size_mb:.1f}MB"
                date_text = str(build.mtime_formatted)
                status_text = Text("Online", style="blue bold")  # Status

            table.add_row(
                selected,
                version_col,
                status_text,
                str(build.branch),  # Ensure all values are strings
                type_text,
                str(hash_text),
                str(size_text),
                str(date_text),
                style=row_style,
            )

    def _add_local_builds_to_table(self, table: Table) -> None:
        """Add local builds to the table.

        Args:
            table: Table to add rows to
        """
        if not self.state.local_builds:
            self.console.print(
                "No local builds found. Press F to fetch online builds.",
                style="bold yellow",
            )
            return

        # Convert local builds dictionary to a sortable list and sort it
        local_build_list = self._get_sorted_local_build_list()

        for i, (version, build_info) in enumerate(local_build_list):
            selected = "[X]" if i in self.state.selected_builds else "[ ]"
            row_style = "reverse bold" if i == self.state.cursor_position else ""
            type_text = (
                self._get_style_for_risk_id(build_info.risk_id)
                if build_info.risk_id
                else ""
            )

            # Get hash if available
            hash_text = getattr(build_info, "hash", "") or ""

            # Format build date nicely if available
            build_date = self._format_build_date(build_info)

            # Get size as string, handle if None
            size_str = (
                f"{build_info.size_mb:.1f}MB" if build_info.size_mb is not None else ""
            )

            table.add_row(
                selected,
                Text(f"■ Blender {version}", style="yellow"),
                Text("Local", style="yellow bold"),  # Status
                str(build_info.branch or ""),
                type_text,
                str(hash_text),
                str(size_str),
                str(build_date),
                style=row_style,
            )

    def _get_sorted_local_build_list(self) -> List[Tuple[str, LocalBuildInfo]]:
        """Get a sorted list of local builds based on current sort settings.

        Returns:
            List of (version, LocalBuildInfo) tuples
        """
        local_build_list = list(self.state.local_builds.items())

        # Define sort key functions for different columns (adjusted for removed # column)
        sort_keys = {
            # No longer using cursor position for selection since we store by version
            2: lambda x: tuple(map(int, x[0].split("."))),  # Version
            3: lambda x: "Local",  # Status (always "Local" for local builds)
            4: lambda x: x[1].branch or "",  # Branch
            5: lambda x: x[1].risk_id or "",  # Type
            6: lambda x: getattr(x[1], "hash", "") or "",  # Hash
            7: lambda x: x[1].size_mb or 0,  # Size
            8: lambda x: x[1].time or "",  # Build Date
        }

        # Use the appropriate sort key if defined, otherwise default to version
        sort_key = sort_keys.get(self.state.sort_column, sort_keys[2])

        # Sort the list
        local_build_list.sort(key=sort_key, reverse=self.state.sort_reverse)

        return local_build_list

    def _get_style_for_risk_id(self, risk_id: Optional[str]) -> Text:
        """Get styled text for risk ID.

        Args:
            risk_id: The risk ID string or None

        Returns:
            Styled Text object
        """
        if not risk_id:
            return Text("")

        risk_styles = {
            "stable": "blue",
            "alpha": "magenta",
            "candidate": "cyan",
        }

        style = risk_styles.get(risk_id, "")
        return Text(risk_id, style=style)

    def _format_build_date(self, build_info: LocalBuildInfo) -> str:
        """Format build date from build info.

        Args:
            build_info: LocalBuildInfo object

        Returns:
            Formatted date string
        """
        if build_info.build_date:
            return build_info.build_date

        if build_info.time and len(build_info.time) == 13:
            try:
                from datetime import datetime

                return datetime.strptime(build_info.time, "%Y%m%d_%H%M").strftime(
                    "%Y-%m-%d %H:%M"
                )
            except ValueError:
                pass

        return build_info.time or "Unknown"

    def print_settings_table(self) -> None:
        """Print a table for settings configuration."""
        # Simpler approach to avoid Rich formatting errors
        self.console.print("[bold]Settings[/bold]")
        self.console.print("─" * 80)

        settings = [
            ("Download Directory:", str(AppConfig.DOWNLOAD_PATH)),
            ("Version Cutoff:", AppConfig.VERSION_CUTOFF),
        ]

        for i, (setting, value) in enumerate(settings):
            # Simple highlighted row for the cursor position
            if i == self.state.settings_cursor:
                self.console.print(
                    f"[reverse bold]> {setting:<20} {value}[/reverse bold]"
                )
            else:
                self.console.print(f"  {setting:<20} {value}")

        # Add some spacing at the bottom
        self.console.print()

    def display_tui(self, full_clear=True) -> None:
        """Display the TUI using Rich's components.

        Args:
            full_clear: Whether to fully clear the screen before redrawing
        """
        # Position cursor at top - this is more gentle than clearing
        self.clear_screen(full_clear=full_clear)

        # Override console width to ensure we have consistent table width
        # This prevents jitter in the display
        table_width = min(self.terminal_width, 120)  # Cap at reasonable width

        if self.state.current_page == "builds":
            if self.state.has_fetched and not self.state.builds:
                self.console.print(
                    "No builds found. Try adjusting version cutoff in Settings.",
                    style="bold yellow",
                )
            else:
                # Print navigation bar at the top
                self.print_navigation_bar()

                # Print the build table (legend will be printed by print_build_table)
                self.print_build_table()
        else:  # settings page
            # Print navigation bar at the top for settings page
            self.print_navigation_bar()

            # Print the settings table
            self.print_settings_table()

        # Clear any remaining content
        self._clear_remaining_lines()

    def _fetch_online_builds(self) -> bool:
        """Fetch builds from the Blender server.

        Returns:
            True to continue running
        """
        # Clear screen to avoid UI interference
        self.clear_screen()
        self.console.print("[bold green]Fetching Blender builds...[/bold green]")

        try:
            self.state.builds = fetch_builds()
            self.state.has_fetched = True
            self.state.cursor_position = 0  # Reset cursor position
            self.console.print("Successfully fetched builds", style="green")
        except Exception as e:
            self.console.print(f"Error fetching builds: {e}", style="bold red")
            return True  # Continue running even if fetch fails

        # After successful fetch, ensure we refresh the display
        self.display_tui()
        return True

    def launch_blender(self) -> Optional[bool]:
        """Launch the selected Blender version.

        Returns:
            True if Blender was launched successfully, False to exit, None to continue
        """
        if not self.state.local_builds:
            self.console.print("No local builds found to launch.", style="bold red")
            return None

        # Get the combined build list
        combined_build_list = self._get_combined_build_list()

        if self.state.cursor_position >= len(combined_build_list):
            self.console.print("Invalid selection.", style="bold red")
            return None

        # Get the build info from the cursor position
        build_type, _, version = combined_build_list[self.state.cursor_position]

        # Check if this version exists locally
        if version not in self.state.local_builds:
            self.console.print(
                f"Blender {version} is not installed locally.", style="bold red"
            )
            return None

        # Find the directory for this version
        build_dir = self._find_build_directory(version)
        if not build_dir:
            self.console.print(
                f"Could not find installation directory for Blender {version}.",
                style="bold red",
            )
            return None

        # Path to the blender executable
        blender_executable = build_dir / "blender"

        if not blender_executable.exists():
            self.console.print(
                f"Blender executable not found at {blender_executable}.",
                style="bold red",
            )
            return None

        # Launch Blender in the background
        try:
            self.console.print(f"Launching Blender {version}...", style="bold green")
            subprocess.Popen([str(blender_executable)], start_new_session=True)
            # Return True to indicate successful launch and exit the application
            return True
        except Exception as e:
            self.console.print(f"Failed to launch Blender: {e}", style="bold red")
            return None

    def _find_build_directory(self, version: str) -> Optional[Path]:
        """Find the build directory for a specific version.

        Args:
            version: Blender version string

        Returns:
            Path to the build directory if found, None otherwise
        """
        download_dir = Path(AppConfig.DOWNLOAD_PATH)

        # Get all directories that start with "blender-"
        all_build_dirs = list(download_dir.glob("blender-*"))

        # Filter to find those matching our version
        for dir_path in all_build_dirs:
            if not dir_path.is_dir():
                continue

            # Extract the version from the directory name
            dir_name = dir_path.name
            if dir_name.startswith(f"blender-{version}"):
                # Direct match at the start
                return dir_path
            elif "-" in dir_name and len(dir_name.split("-")) > 1:
                # Check if the part after "blender-" is our version
                dir_version = dir_name.split("-")[1]

                # Handle case where version might contain a +
                if "+" in dir_version:
                    dir_version = dir_version.split("+")[0]

                if dir_version == version:
                    return dir_path

        return None

    def sort_builds(self, builds: List[BlenderBuild]) -> List[BlenderBuild]:
        """Sort builds based on current sort configuration.

        Args:
            builds: List of builds to sort

        Returns:
            Sorted list of builds
        """
        if not builds:
            return []

        # Make a copy to not modify the original list
        sorted_builds = builds.copy()

        # Define sort keys for different columns
        sort_keys = {
            # Adjusted column indices (after removing # column)
            # No longer using cursor position for selection since we store by version
            2: lambda b: tuple(map(int, b.version.split("."))),  # Version
            3: lambda b: "Online",  # Status (always "Online" for online builds)
            4: lambda b: b.branch,  # Branch
            5: lambda b: b.risk_id,  # Type
            6: lambda b: b.hash or "",  # Hash - default to empty string if None
            7: lambda b: b.size_mb,  # Size
            8: lambda b: b.file_mtime,  # Build Date
        }

        # Use the appropriate sort key if defined, otherwise don't sort
        if self.state.sort_column in sort_keys:
            sorted_builds.sort(
                key=sort_keys[self.state.sort_column], reverse=self.state.sort_reverse
            )

        return sorted_builds

    def run(self) -> None:
        """Run the TUI application."""
        try:
            # Initial display
            self.clear_screen()
            self.display_tui()

            # Main input loop
            running = True
            last_refresh_time = time.time()
            last_full_refresh = time.time()

            # Track UI state to minimize refreshes
            is_download_in_progress = False

            while running:
                try:
                    # Get key input with our custom key handling
                    # Use a shorter timeout when downloads are in progress to keep the UI responsive
                    # but not so short that it causes excessive refreshing
                    timeout = 0.1 if self.state.download_progress else 0.5

                    # Show cursor when waiting for input
                    print("\033[?25h", end="", flush=True)
                    key = get_keypress(timeout=timeout)
                    # Hide cursor again when displaying UI
                    print("\033[?25l", end="", flush=True)

                    current_time = time.time()

                    # Check for download state changes
                    download_active = bool(self.state.download_progress)
                    if download_active != is_download_in_progress:
                        is_download_in_progress = download_active

                        # Always clear screen and display the updated table
                        self.clear_screen()
                        self.display_tui()

                        last_refresh_time = current_time
                        last_full_refresh = current_time
                        continue

                    # If no key was pressed, check if we need a refresh for download progress
                    if key is None:
                        if self.state.needs_refresh:
                            # For downloads, update at a reasonable rate to prevent flashing
                            time_since_last_refresh = current_time - last_refresh_time
                            if (
                                time_since_last_refresh >= 0.75
                            ):  # Reduce refresh rate to once per 0.75 seconds
                                # Always use the standard display method instead of the specialized download one
                                self.display_tui(full_clear=False)
                                self.state.needs_refresh = False
                                last_refresh_time = current_time
                        continue

                    # Handle the key based on the current page
                    handler = self.page_handlers.get(self.state.current_page)
                    if handler:
                        running = handler(key)

                        # Refresh display after handling key without a full clear
                        self.display_tui(full_clear=False)
                        last_refresh_time = time.time()
                        last_full_refresh = time.time()

                except Exception as inner_e:
                    self.console.print(
                        f"Error handling input: {inner_e}", style="bold red"
                    )
                    self.console.print(f"Type: {type(inner_e)}")
                    self.display_tui()
                    last_refresh_time = time.time()

        except Exception as e:
            self.console.print(f"An error occurred: {e}", style="bold red")
        finally:
            # Always ensure cursor is visible when program exits
            print("\033[?25h", end="", flush=True)

    def _handle_builds_page_input(self, key: str) -> bool:
        """Handle input for the builds page.

        Args:
            key: The pressed key

        Returns:
            False to exit the application, True to continue
        """
        # Map keys to handlers with string constants
        key_handlers = {
            KEY_UP: self._move_cursor_up,
            KEY_DOWN: self._move_cursor_down,
            KEY_LEFT: self._move_column_left,
            KEY_RIGHT: self._move_column_right,
            KEY_ENTER: self._launch_blender,
            KEY_ENTER_ALT: self._launch_blender,  # Handle both Enter key types
            KEY_SPACE: self._toggle_selection,
            "k": self._move_cursor_up,
            "j": self._move_cursor_down,
            "h": self._move_column_left,
            "l": self._move_column_right,
            "r": self._toggle_sort_reverse,
            "f": self._fetch_online_builds,
            "s": self._switch_to_settings,
            "q": lambda: False,  # Return False to exit
            "d": self._handle_download,
            "x": self._delete_selected_build,
            "\x03": lambda: False,  # Ctrl-C (ASCII value 3) to exit
        }

        # Check if key is in our handlers
        if key in key_handlers:
            return key_handlers[key]()

        # Handle letter keys (case-insensitive)
        if isinstance(key, str) and len(key) == 1 and key.isalpha():
            key_lower = key.lower()
            if key_lower in key_handlers:
                return key_handlers[key_lower]()

        return True  # Continue running by default

    def _handle_settings_page_input(self, key: str) -> bool:
        """Handle input for the settings page.

        Args:
            key: The pressed key

        Returns:
            False to exit the application, True to continue
        """
        # Map keys to handlers with string constants
        key_handlers = {
            KEY_UP: lambda: self._move_settings_cursor(-1),
            KEY_DOWN: lambda: self._move_settings_cursor(1),
            KEY_ENTER: self._edit_current_setting,
            KEY_ENTER_ALT: self._edit_current_setting,
            "k": lambda: self._move_settings_cursor(-1),
            "j": lambda: self._move_settings_cursor(1),
            "s": self._switch_to_builds_page,
            "q": lambda: False,  # Return False to exit
            "\x03": lambda: False,  # Ctrl-C (ASCII value 3) to exit
        }

        # Check if key is in our handlers
        if key in key_handlers:
            return key_handlers[key]()

        # Handle letter keys (case-insensitive)
        if isinstance(key, str) and len(key) == 1 and key.isalpha():
            key_lower = key.lower()
            if key_lower in key_handlers:
                return key_handlers[key_lower]()

        return True  # Continue running by default

    def _move_settings_cursor(self, direction: int) -> bool:
        """Move settings cursor up or down.

        Args:
            direction: -1 for up, 1 for down

        Returns:
            True to continue running
        """
        if direction < 0:
            # Move up
            self.state.settings_cursor = max(0, self.state.settings_cursor - 1)
        else:
            # Move down (only 2 settings now)
            self.state.settings_cursor = min(self.state.settings_cursor + 1, 1)

        # Use partial screen update to reduce flashing
        self.display_tui(full_clear=False)
        return True

    def _switch_to_builds_page(self) -> bool:
        """Switch back to builds page.

        Returns:
            True to continue running
        """
        self.state.current_page = "builds"
        self.display_tui()
        return True

    def _move_cursor_up(self) -> bool:
        """Move cursor up.

        Returns:
            True to continue running
        """
        self.state.cursor_position = max(0, self.state.cursor_position - 1)
        # Use partial screen update to reduce flashing
        self.display_tui(full_clear=False)
        return True

    def _move_cursor_down(self) -> bool:
        """Move cursor down.

        Returns:
            True to continue running
        """
        # Calculate the total number of items in the combined list
        total_items = len(self._get_combined_build_list())
        max_index = total_items - 1 if total_items > 0 else 0

        self.state.cursor_position = min(self.state.cursor_position + 1, max_index)
        # Use partial screen update to reduce flashing
        self.display_tui(full_clear=False)
        return True

    def _get_combined_build_list(self) -> List[Tuple[str, int, str]]:
        """Get a combined list of all builds (local and online).

        Returns:
            List of tuples (type, index, version)
        """
        combined_list = []

        # First add all local builds
        for i, (version, build_info) in enumerate(self._get_sorted_local_build_list()):
            combined_list.append(("local", i, version))

        # Track local versions to avoid duplicates
        local_versions = set(version for _, _, version in combined_list)

        # Then add online builds that aren't local
        if self.state.has_fetched and self.state.builds:
            sorted_builds = self.sort_builds(self.state.builds)
            for i, build in enumerate(sorted_builds):
                if build.version not in local_versions:
                    combined_list.append(("online", i, build.version))

        # Sort the entire list by version if sorting by version
        if self.state.sort_column == 2:  # Version column (adjusted index)

            def version_sort_key(item):
                # Split version string into components and convert to integers
                return tuple(map(int, item[2].split(".")))

            combined_list.sort(key=version_sort_key, reverse=self.state.sort_reverse)

        return combined_list

    def _move_column_left(self) -> bool:
        """Select previous column.

        Returns:
            True to continue running
        """
        if self.state.sort_column > 1:  # Start from column 1 now (Selection)
            self.state.sort_column -= 1
            # Use partial screen update to reduce flashing
            self.display_tui(full_clear=False)
        return True

    def _move_column_right(self) -> bool:
        """Select next column.

        Returns:
            True to continue running
        """
        if self.state.sort_column < 8:  # Adjust max column index (was 7)
            self.state.sort_column += 1
            # Use partial screen update to reduce flashing
            self.display_tui(full_clear=False)
        return True

    def _toggle_sort_reverse(self) -> bool:
        """Reverse sort order.

        Returns:
            True to continue running
        """
        self.state.sort_reverse = not self.state.sort_reverse
        # Use partial screen update to reduce flashing
        self.display_tui(full_clear=False)
        return True

    def _toggle_selection(self) -> bool:
        """Toggle selection state of the current item.

        Returns:
            True to continue running
        """
        if not self._has_visible_builds():
            return True

        # Get the version of the build at the current cursor position
        combined_build_list = self._get_combined_build_list()
        if 0 <= self.state.cursor_position < len(combined_build_list):
            _, _, version = combined_build_list[self.state.cursor_position]

            # Toggle selection based on version instead of cursor position
            if version in self.state.selected_builds:
                self.state.selected_builds.remove(version)
            else:
                self.state.selected_builds.add(version)

            # Use partial screen update to reduce flashing
            self.display_tui(full_clear=False)
        return True

    def _has_visible_builds(self) -> bool:
        """Check if there are any builds visible in the current view."""
        return len(self._get_combined_build_list()) > 0

    def _switch_to_settings(self) -> bool:
        """Switch to settings page.

        Returns:
            True to continue running
        """
        self.state.current_page = "settings"
        self.display_tui()
        return True

    def _launch_blender(self) -> bool:
        """Launch the selected Blender version.

        Returns:
            False to exit (only on builds page), True to continue
        """
        # Only attempt to launch Blender if we're on the builds page
        if self.state.current_page == "builds":
            result = self.launch_blender()
            # If launch was successful, exit the application
            return False if result else True

        # On settings page, just continue running
        return True

    def _handle_download(self) -> bool:
        """Download selected builds and show progress in the table.

        Returns:
            True to continue running
        """
        if not self.state.has_fetched or not self.state.builds:
            self.console.print("No online builds to download", style="yellow")
            return True

        builds_to_download = []
        download_indices = []

        # Get the combined build list to determine what's selected
        combined_build_list = self._get_combined_build_list()

        if self.state.selected_builds:
            # Get the builds that are currently selected by version
            for version in self.state.selected_builds:
                # Find this version in online builds
                for build in self.state.builds:
                    if build.version == version:
                        # Only download if it's not local or has an update
                        if (
                            version not in self.state.local_builds
                            or self._has_update_available(version)
                        ):
                            builds_to_download.append(build)
                            # Find the index in the combined list for progress tracking
                            for i, (_, _, v) in enumerate(combined_build_list):
                                if v == version:
                                    download_indices.append(i)
                                    break
                        break
        else:
            # If no builds are selected, download the one under the cursor
            if 0 <= self.state.cursor_position < len(combined_build_list):
                build_type, orig_idx, version = combined_build_list[
                    self.state.cursor_position
                ]
                # Find the corresponding online build
                for build in self.state.builds:
                    if build.version == version:
                        builds_to_download.append(build)
                        download_indices.append(self.state.cursor_position)
                        break

        if not builds_to_download:
            self.console.print("No builds selected for download", style="yellow")
            return True

        # Initialize download tracking
        self.state.download_progress = {}
        self.state.download_speed = {}
        for build in builds_to_download:
            self.state.download_progress[build.version] = 0
            self.state.download_speed[build.version] = "Starting..."

        # Create a panel with the download message
        from rich.panel import Panel
        from rich.align import Align

        panel = Panel(
            Align.center("Preparing download...please wait"),
            border_style="green bold",
            width=40,
        )

        # Display the centered panel
        self._display_centered_panel(panel)

        # Start download in background thread
        import threading

        download_thread = threading.Thread(
            target=self._download_with_progress_tracking,
            args=(builds_to_download,),
            daemon=True,
        )
        download_thread.start()

        # Return to main loop, which will detect the download is active
        # and update the UI accordingly
        return True

    def _download_with_progress_tracking(self, builds: List[BlenderBuild]) -> None:
        """Download builds and track progress for display in the main table.

        Args:
            builds: The builds to download
        """
        import time
        import threading
        import os
        from ..utils.download import (
            download_multiple_builds,
            _get_progress_and_speed_from_log,
            get_log_file_path,
        )

        # Start the downloads
        download_thread = threading.Thread(
            target=download_multiple_builds, args=(builds,), daemon=True
        )
        download_thread.start()

        # Get log file paths
        temp_log_files = {}
        for build in builds:
            temp_log_files[build.version] = get_log_file_path(build)

        # Track previous progress values
        previous_progress = {}
        for build in builds:
            previous_progress[build.version] = -1

        # Track the last time we requested a UI refresh
        last_refresh_request = time.time()

        try:
            # Give download_multiple_builds a chance to create log files
            time.sleep(1)

            running = True
            completed_builds = set()

            while running and download_thread.is_alive():
                any_progress_updated = False
                significant_change = False
                current_time = time.time()

                for build in builds:
                    if build.version in completed_builds:
                        continue

                    log_file = temp_log_files[build.version]
                    if os.path.exists(log_file):
                        result = _get_progress_and_speed_from_log(log_file)
                        if result:
                            percentage, speed = result

                            # Check if the progress has changed significantly
                            # For smoother UI, consider smaller changes significant as progress increases
                            progress_delta = abs(
                                percentage - previous_progress[build.version]
                            )
                            if (
                                progress_delta >= 5
                                or previous_progress[build.version] == -1
                                or (
                                    percentage == 100
                                    and previous_progress[build.version] < 100
                                )
                            ):
                                significant_change = True
                                previous_progress[build.version] = percentage

                            # No need to check for significant changes - always update
                            # to ensure smooth progress display
                            previous_progress[build.version] = percentage

                            # Update progress tracking
                            self.state.download_progress[build.version] = percentage
                            self.state.download_speed[build.version] = speed
                            any_progress_updated = True

                            # Mark as completed if download is done
                            if percentage >= 100:
                                completed_builds.add(build.version)
                                self.state.download_speed[build.version] = (
                                    "Extracting..."
                                )

                # Tell main loop to update the UI if there's a significant change
                # and sufficient time has passed since last refresh
                if (significant_change or any_progress_updated) and (
                    current_time - last_refresh_request >= 0.5
                ):
                    self.state.needs_refresh = True
                    last_refresh_request = current_time

                # Check if we should exit the loop
                if not any_progress_updated and not download_thread.is_alive():
                    running = False

                # Don't check too frequently to avoid excess CPU usage
                time.sleep(0.5)

            # Final update
            for build in builds:
                self.state.download_progress[build.version] = 100
                self.state.download_speed[build.version] = "Complete"

            # Force refresh to show final status
            self.state.needs_refresh = True

            # Wait a bit for the user to see completion
            time.sleep(1)

            # Update local builds with any new downloads
            self.state.local_builds = get_local_builds()

            # Clean up and force final refresh
            self.state.download_progress = {}
            self.state.download_speed = {}
            self.state.needs_refresh = True

        except Exception as e:
            # Show error
            for build in builds:
                self.state.download_progress[build.version] = 0
                self.state.download_speed[build.version] = f"Error: {e}"

            # Force refresh to show error status
            self.state.needs_refresh = True
            time.sleep(2)

            # Clean up
            self.state.download_progress = {}
            self.state.download_speed = {}
            self.state.needs_refresh = True

    def _has_update_available(self, version: str) -> bool:
        """Check if an update is available for a local build.

        Args:
            version: The version string to check

        Returns:
            True if an update is available, False otherwise
        """
        if not self.state.has_fetched or version not in self.state.local_builds:
            return False

        local_build = self.state.local_builds[version]

        # Find the corresponding online build
        for build in self.state.builds:
            if build.version == version:
                # Check if the build times differ
                if (
                    local_build.time
                    and build.build_time
                    and local_build.time != build.build_time
                ):
                    return True
                break

        return False

    def _edit_current_setting(self) -> bool:
        """Edit the currently selected setting.

        Returns:
            True to continue running
        """
        try:
            self.clear_screen()

            if self.state.settings_cursor == 0:  # Download path
                self._edit_download_path()
            elif self.state.settings_cursor == 1:  # Version cutoff
                self._edit_version_cutoff()

            self.display_tui()
            return True  # Return True to prevent application exit
        except ValueError as e:
            self.console.print(f"Error updating settings: {e}", style="bold red")
            self.display_tui()
            return True  # Return True to prevent application exit

    def _edit_download_path(self) -> None:
        """Edit the download path setting."""
        current = AppConfig.DOWNLOAD_PATH

        # Show cursor for text input
        print("\033[?25h", end="", flush=True)
        new_path = prompt_input(
            f"Enter new download path [cyan]({current})[/cyan]", default=str(current)
        )
        # Hide cursor after input
        print("\033[?25l", end="", flush=True)

        if new_path and new_path != str(current):
            try:
                # Update the class attribute directly
                AppConfig.DOWNLOAD_PATH = Path(new_path)
                AppConfig.save_config()  # Assuming there's a class method to save
                self.state.needs_refresh = True
            except Exception as e:
                self.console.print(
                    f"Failed to update download path: {e}", style="bold red"
                )

    def _edit_version_cutoff(self) -> None:
        """Edit the version cutoff setting."""
        current = AppConfig.VERSION_CUTOFF

        # Show cursor for text input
        print("\033[?25h", end="", flush=True)
        new_cutoff = prompt_input(
            f"Enter minimum Blender version to display [cyan]({current})[/cyan]",
            default=current,
        )
        # Hide cursor after input
        print("\033[?25l", end="", flush=True)

        if new_cutoff and new_cutoff != current:
            try:
                AppConfig.VERSION_CUTOFF = new_cutoff
                AppConfig.save_config()
                self.state.needs_refresh = True
            except Exception as e:
                self.console.print(
                    f"Failed to update version cutoff: {e}", style="bold red"
                )

    def _delete_selected_build(self) -> bool:
        """Delete the selected local build(s).

        If multiple builds are selected via toggle, delete them all.

        Returns:
            True to continue running
        """
        if not self.state.local_builds:
            return True

        # Get the complete list of builds (both local and online if fetched)
        combined_build_list = self._get_combined_build_list()

        versions_to_delete = []
        if self.state.selected_builds:
            # Delete all selected builds
            for version in self.state.selected_builds:
                if version in self.state.local_builds:
                    versions_to_delete.append(version)
        else:
            # No selection, delete the build at the current cursor
            if 0 <= self.state.cursor_position < len(combined_build_list):
                build_type, _, version = combined_build_list[self.state.cursor_position]
                if version in self.state.local_builds:
                    versions_to_delete.append(version)

        if not versions_to_delete:
            return True

        # Show deletion confirmation panel
        from rich.panel import Panel
        from rich.align import Align

        prompt_text = f"Delete Blender {', '.join(versions_to_delete)}? (y/n)"
        panel = Panel(
            Align.center(prompt_text),
            title="Confirm Deletion",
            border_style="red bold",
            width=min(len(prompt_text) + 10, 80),
        )

        # Display the centered panel
        self._display_centered_panel(panel)

        # Get user confirmation
        while True:
            key = get_keypress(timeout=None)
            if key is not None:
                break

        if key.lower() != "y":
            self.display_tui()  # Redraw the UI
            return True

        # Perform deletion
        success = True
        for version in versions_to_delete:
            if not delete_local_build(version):
                success = False

        # Update state
        self.state.selected_builds.clear()
        self.state.local_builds = get_local_builds()

        # Update cursor position
        max_index = len(self._get_combined_build_list()) - 1
        if max_index < 0:
            max_index = 0
        self.state.cursor_position = min(self.state.cursor_position, max_index)

        # Show result panel
        result_text = (
            f"Deleted Blender {'builds' if len(versions_to_delete) > 1 else version}"
            if success
            else "Failed to delete one or more builds"
        )
        result_panel = Panel(
            Align.center(result_text),
            title="Result",
            border_style="green bold" if success else "red bold",
            width=min(len(result_text) + 10, 80),
        )

        # Display the centered result panel
        self._display_centered_panel(result_panel)

        # Brief pause before returning to normal view
        time.sleep(1.5)
        self.display_tui()

        return True

    def _get_current_build_list(
        self,
    ) -> Union[List[BlenderBuild], List[Tuple[str, LocalBuildInfo]]]:
        """Get the currently visible build list based on view state.

        Returns:
            Either a list of BlenderBuild objects or a list of (version, LocalBuildInfo) tuples
        """
        if self.state.has_fetched:
            # When looking at online builds, we need the sorted list
            return self.sort_builds(self.state.builds)
        else:
            # When looking at local builds, get the sorted list
            return self._get_sorted_local_build_list()

    def display_download_progress(self) -> None:
        """Optimized display method for download progress updates.
        Only updates the essential elements to minimize screen updates and flashing.
        """
        # Position cursor at home position without clearing the screen
        print("\033[H", end="", flush=True)
        print("\033[?25l", end="", flush=True)

        # Update terminal size before rendering
        self.terminal_width, self.terminal_height = self.console.size

        # Print navigation bar at the top
        self.print_navigation_bar()

        # Create a minimalist table just showing downloads
        base_columns = [
            "",  # Selection column
            "Version",
            "Status",
            "Branch",
            "Type",
            "Hash",
            "Size",
        ]

        # Apply responsive column hiding based on terminal width
        column_names = self._get_responsive_columns(base_columns)

        # Always add the Speed column for downloads
        column_names.append("Speed")

        table = self._create_table(column_names)

        # Only add rows for builds being downloaded
        combined_build_list = self._get_combined_build_list()

        # For each build that's being downloaded
        for i, (build_type, original_idx, version) in enumerate(combined_build_list):
            if version in self.state.download_progress:
                selected = "[X]"  # Downloaded builds are selected
                row_style = ""

                # Find the build info
                online_build = None
                for build in self.state.builds:
                    if build.version == version:
                        online_build = build
                        break

                if online_build:
                    # Get download progress and speed
                    download_progress = self.state.download_progress.get(version, 0)
                    download_speed = self.state.download_speed.get(version, "")

                    # Set values for the table row
                    version_col = Text(f"↓ Blender {version}", style="green bold")
                    status_text = Text("Downloading", style="green bold")
                    type_text = Text("downloading", style="green bold")
                    hash_text = online_build.hash or ""
                    size_text = f"{download_progress:.0f}%"
                    date_text = download_speed

                    # Prepare row data
                    row_data = [
                        selected,
                        version_col,
                        status_text,
                    ]

                    # Only add Branch column if it's in our responsive columns
                    if "Branch" in column_names:
                        row_data.append(str(online_build.branch))

                    row_data.append(type_text)

                    # Only add Hash column if it's in our responsive columns
                    if "Hash" in column_names:
                        row_data.append(str(hash_text))

                    # Only add Size column if it's in our responsive columns
                    if "Size" in column_names:
                        row_data.append(str(size_text))

                    row_data.append(str(date_text))

                    table.add_row(*row_data, style=row_style)

        # Print the table
        self.console.print(table)

        # Display legend below the table
        legend = Text()
        legend.append("■ Current Local Version", style="yellow")
        legend.append("   ")
        legend.append("■ Update Available", style="green")
        self.console.print(legend)

        # Add cancel download hint
        self.console.print("Press any key to cancel download", style="dim")

        # Clear any remaining content
        self._clear_remaining_lines()

    def _display_centered_panel(self, panel) -> None:
        """Display a panel centered both horizontally and vertically.

        Args:
            panel: Rich Panel object to display centered
        """
        # Clear screen first - this needs a full clear for panels
        self.clear_screen(full_clear=True)

        # Print navigation at the top
        self.print_navigation_bar()

        # Calculate vertical position to center the panel
        terminal_height = self.console.height
        # Estimate panel height as 3 lines for simple panels, adjust if needed
        panel_height = 3
        nav_bar_height = 2  # Navigation bar takes about 2 lines

        vertical_padding = max(
            0, (terminal_height - panel_height - nav_bar_height) // 2
        )

        # Add vertical padding
        for _ in range(vertical_padding):
            self.console.print("")

        # Print centered panel
        from rich.align import Align

        self.console.print(Align.center(panel))
