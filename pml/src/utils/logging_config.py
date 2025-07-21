import logging
import os
import sys
from typing import Literal
from colorama import Fore, Style, init

class ConsoleFormatter(logging.Formatter):
    """
    A custom log formatter for console output that adds emojis and colors.
    It prepends a colored emoji and log level to the log message.
    """

    LEVEL_CONFIG = {
        logging.DEBUG:    {"color": Fore.CYAN, "emoji": "🐛", "label": "DEBUG"},
        logging.INFO:     {"color": Fore.GREEN, "emoji": "✨", "label": "INFO"},
        logging.WARNING:  {"color": Fore.YELLOW, "emoji": "⚠️ ", "label": "WARNING"},
        logging.ERROR:    {"color": Fore.RED, "emoji": "❌", "label": "ERROR"},
        logging.CRITICAL: {"color": Fore.RED + Style.BRIGHT, "emoji": "🔥", "label": "CRITICAL"},
    }

    def __init__(self, fmt="%(name)s - %(message)s", datefmt=None, style: Literal['%', '{', '$'] = '%'):
        super().__init__(fmt=fmt, datefmt=datefmt, style=style)
        init(autoreset=True)

    def format(self, record):
        # Let the parent class do the initial formatting of the message
        log_str = super().format(record)

        # Get configuration for the specific log level
        config = self.LEVEL_CONFIG.get(record.levelno, {})
        color = config.get("color", "")
        emoji = config.get("emoji", "➡️")  # Default emoji for custom levels
        label = config.get("label", record.levelname)

        # Construct the colored/emoji prefix
        prefix = f"{color}{emoji} [{label}]{Style.RESET_ALL}"

        # Prepend the prefix to the formatted message string
        return f"{prefix} {log_str}"

def setup_logging(
    log_level=logging.INFO,
    log_file: str | None = "pml_service.log",
    console: bool = True
):
    """
    Set up logging for the application.

    This function configures a root logger with two handlers:
    1. A console handler that outputs colored logs with emojis.
    2. A file handler that outputs plain text logs to a specified file.

    Args:
        log_level: The minimum logging level to process (e.g., logging.DEBUG).
        log_file: Path to the log file. If None, file logging is disabled.
                  Defaults to "pml_service.log".
        console: Whether to enable console logging. Defaults to True.
    """
    logger = logging.getLogger()
    logger.setLevel(log_level)

    # Clear any existing handlers to prevent duplicate logs if this is called multiple times
    if logger.hasHandlers():
        logger.handlers.clear()

    # Configure console handler
    if console:
        console_handler = logging.StreamHandler(sys.stdout)
        # The format string for the console does not include levelname,
        # as the custom formatter prepends it with color and emoji.
        console_formatter = ConsoleFormatter(
            fmt="%(name)s: %(message)s"
        )
        console_handler.setFormatter(console_formatter)
        logger.addHandler(console_handler)

    # Configure file handler
    if log_file:
        try:
            # Ensure the directory for the log file exists
            log_dir = os.path.dirname(os.path.abspath(log_file))
            if not os.path.exists(log_dir):
                os.makedirs(log_dir)

            file_handler = logging.FileHandler(log_file, mode='a', encoding='utf-8')
            # The file formatter uses a standard, detailed format without color or emoji.
            file_formatter = logging.Formatter(
                fmt='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
                datefmt='%Y-%m-%d %H:%M:%S'
            )
            file_handler.setFormatter(file_formatter)
            logger.addHandler(file_handler)
        except (IOError, OSError) as e:
            # Use a basic logger for this error since the handler might have failed
            logging.basicConfig()
            logging.error(f"Failed to configure file logging for {log_file}: {e}")

    logging.info("Logging has been configured successfully.")


if __name__ == '__main__':
    # This block demonstrates how to use the logging setup
    print("--- Running logging_config.py example ---")

    # Configure logging with DEBUG level and a test file
    setup_logging(log_level=logging.DEBUG, log_file="test_log.log")

    # Get a logger instance for the current module
    module_logger = logging.getLogger(__name__)

    module_logger.debug("This is a debug message.")
    module_logger.info("This is an info message.")
    module_logger.warning("This is a warning message.")
    module_logger.error("This is an error message.")
    module_logger.critical("This is a critical message from the module logger.")

    # Demonstrate with a different logger name to see the name in the output
    another_logger = logging.getLogger("Another.Component")
    another_logger.info("This message comes from another named component.")

    print("\n--- Example finished. Check 'test_log.log' for the plain text file output. ---")
