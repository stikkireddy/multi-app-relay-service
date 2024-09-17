import os
import subprocess
from typing import List

def ensure_python_path(env):
    import sys
    from pathlib import Path
    for python_version_dir in (Path(sys.executable).parent.parent / "lib").iterdir():
        site_packages = str(python_version_dir / "site-packages")
        py_path = env.get("PYTHONPATH", "")
        if site_packages not in py_path.split(":"):
            env["PYTHONPATH"] = f"{py_path}:{site_packages}"

def execute(cmd: List[str], env, cwd=None, ensure_python_site_packages=True, shell=False, trim_new_line=True):
    if ensure_python_site_packages:
        ensure_python_path(env)
    import subprocess
    if shell is True:
        cmd = " ".join(cmd)
    popen = subprocess.Popen(cmd,
                             stdout=subprocess.PIPE,
                             stderr=subprocess.STDOUT,
                             universal_newlines=True,
                             shell=shell,
                             env=env,
                             cwd=cwd,
                             bufsize=1)
    if popen.stdout is not None:
        for stdout_line in iter(popen.stdout.readline, ""):
            if trim_new_line:
                stdout_line = stdout_line.strip()
            yield stdout_line


    popen.stdout.close()
    # popen.stderr.close()  # Close stderr
    return_code = popen.wait()
    if return_code:
        raise subprocess.CalledProcessError(return_code, cmd)

port = os.environ.get("PORT", 9989)

class CodeServerTunnel():

    def _install_databricks_cli(self):
        print("Attempting to install databricks cli")
        command = "curl -fsSL https://raw.githubusercontent.com/databricks/setup-cli/main/install.sh | sh"
        env_copy = os.environ.copy()
        already_installed = False
        try:
            for stmt in execute([command], shell=True, env=env_copy):
                if "already exists" in stmt:
                    already_installed = True
                print(stmt)
        except subprocess.CalledProcessError as e:
            if already_installed is False:
                raise e
        print("Finished installing databricks cli")

    def _install_extension(self, env, extension_id: str):
        for stmt in execute(["code-server", "--install-extension", extension_id], shell=True, env=env):
            print(stmt)
        print(f"Finished Installed extension: {extension_id}")

    def _install_extensions(self, env, extensions: list[str]):
        for extension in extensions:
            self._install_extension(env, extension)

    def run(self):
        import subprocess

        import os
        import subprocess
        print("It may take a 15-30 seconds for the code server to start up.")

        print("Installing code server")
        url = "https://code-server.dev/install.sh"
        # Equivalent Python subprocess command with piping
        os.environ["HOME"] = os.getcwd()
        print(f"Working in home dir {os.getcwd()}")
        subprocess.run(f'wget https://nodejs.org/dist/v20.5.1/node-v20.5.1-linux-x64.tar.xz', check=True, shell=True)
        subprocess.run(f'tar -xf node-v20.5.1-linux-x64.tar.xz', check=True, shell=True)
        os.environ["PATH"] = os.getcwd()+"/node-v20.5.1-linux-x64/bin:"+os.environ["PATH"]
        subprocess.run(f'npm install -g node-gyp', check=True, shell=True)
        subprocess.run(f'npm install -g code-server', check=True, shell=True)
        print("Installed code server")

        # self._install_databricks_cli()

        my_env = os.environ.copy()
        my_env["VSCODE_PROXY_URI"] = "./relay/vscode/wss"
        subprocess.run(f"kill -9 $(lsof -t -i:{port})", capture_output=True, shell=True)

        print(f"Installing default plugins!")

        print(f"Deploying code server on port: {port}")
        cmd = ["code-server",
               "--bind-addr",
               f"0.0.0.0:{port}",
               "--auth",
               "none"]
        print(f"Running command: {' '.join(cmd)}")
        for path in execute(cmd, my_env):
            print(path)

if __name__ == "__main__":
    CodeServerTunnel().run()
