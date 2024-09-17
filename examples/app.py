import os
import subprocess
from typing import List

default_path = "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:/app/python/source_code/.venv/bin"

# setup path incase it does not exist
if os.getenv("PATH", None) is None:
    os.environ["PATH"] = default_path

if "/bin" not in os.environ["PATH"]:
    os.environ["PATH"] = default_path + ":" + os.environ["PATH"]


class CommandRunner:
    @staticmethod
    def run(command: List[str]) -> subprocess.CompletedProcess:
        env = os.environ.copy()
        env['PYTHONUNBUFFERED'] = '1'

        with subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, env=env) as process:
            try:
                # Loop to read stdout in real-time
                for line in process.stdout:
                    print(line, end='')  # Print each line as it comes in

                # Wait for the process to complete
                process.wait()

                if process.returncode != 0:
                    raise subprocess.CalledProcessError(process.returncode, command)

                return subprocess.CompletedProcess(process.args, process.returncode, process.stdout, process.stderr)

            except Exception as e:
                process.kill()
                process.wait()
                raise e


if __name__ == "__main__":
    import sys
    python_executable = sys.executable  # Get the path to the Python interpreter
    print(f"Python executable: {python_executable}")
    file_path = os.getcwd()+"/mars"
    print("starting app")
    os.chmod(file_path, os.stat(file_path).st_mode | 0o111)
    CommandRunner.run([file_path])