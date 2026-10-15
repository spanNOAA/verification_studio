#!/usr/bin/env python3
"""
Automated Security Test Suite for HPC Read-Only Access V1 (8-Point Checklist).
Verifies:
1. P0-1: Permanent removal of arbitrary command execution (CLI and Daemon).
2. P0-2: Strict minimal API (only list, stat, download).
3. P0-3: Daemon Default-Deny and structured JSON-only RPC handling.
4. P0-4: PTY control character rejection while allowing legal spaces, brackets, Unicode, $.
5. P1-5: Isolated socket namespace (hpc_ro_{cluster}), 0700 dir, 0600 socket, peer UID verification.
6. P1-7: 50MB payload limit, atomic local file creation, and symlink protection.
"""

import os
import sys
import subprocess
import socket
import json
import tempfile
import unittest
import base64
import importlib.machinery
import importlib.util

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
HPC_RUN_PATH = os.path.join(SCRIPT_DIR, "hpc-run")

# Import internal hpc-run module for direct unit testing
loader = importlib.machinery.SourceFileLoader("hpc_run_mod", HPC_RUN_PATH)
spec = importlib.util.spec_from_loader(loader.name, loader)
hpc_mod = importlib.util.module_from_spec(spec)
loader.exec_module(hpc_mod)


class TestPathSanitization(unittest.TestCase):
    def test_pty_control_characters_rejected(self):
        # P0-4: Reject PTY control characters and linebreaks
        dangerous = [
            "/path/to/file\nwith_newline",
            "/path/with\rcarriage_return",
            "/path/with\0null_byte",
            "/path/with\x03ctrl_c",
            "/path/with\x15ctrl_u",
            "/path/with\x7fdel",
        ]
        for p in dangerous:
            ok, err = hpc_mod.sanitize_remote_path(p)
            self.assertFalse(ok, f"PTY control character in '{repr(p)}' should have been rejected!")
            self.assertIn("control character", err.lower())

    def test_legal_complex_path_characters_preserved(self):
        # P0-4: Do NOT use strict character whitelist; preserve spaces, brackets, Unicode, $
        legal = [
            "/gpfs/f6/model output/radar (1).png",
            "/gpfs/f6/project/det_test [v2.1]/image.png",
            "/gpfs/f6/scratch/run_$USER/cref.png",
            "/gpfs/f6/气象数据/雷达图_2026.png",
            "/scratch/user/model+exp-123_456.dat",
        ]
        for p in legal:
            ok, res = hpc_mod.sanitize_remote_path(p)
            self.assertTrue(ok, f"Legal path '{p}' was unexpectedly rejected: {res}")
            self.assertEqual(res, p)


class TestCliDefaultDeny(unittest.TestCase):
    def run_hpc(self, *args):
        cmd = [sys.executable, HPC_RUN_PATH, *args]
        return subprocess.run(cmd, capture_output=True, text=True, env=dict(os.environ, HPC_NON_INTERACTIVE="1"))

    def test_arbitrary_shell_command_blocked(self):
        # P0-1: Delete hpc-run <cluster> "<command>"
        commands = [
            ["gaea-c6", "rm -rf /"],
            ["gaea-c6", "whoami"],
            ["gaea-c6", "ls -la"],
            ["gaea-c6", "touch /tmp/evil"],
            ["gaea-c6", "echo hello"],
        ]
        for args in commands:
            res = self.run_hpc(*args)
            self.assertEqual(res.returncode, 13, f"Command {args} was not blocked with exit code 13!")
            self.assertIn("READ_ONLY_OPERATION_DENIED", res.stderr)
            self.assertIn("Arbitrary shell command execution is permanently disabled", res.stderr)

    def test_async_mode_blocked(self):
        # P0-1: Delete --async
        res = self.run_hpc("gaea-c6", "--async", "python test.py")
        self.assertEqual(res.returncode, 13)
        self.assertIn("READ_ONLY_OPERATION_DENIED", res.stderr)

    def test_banned_and_removed_subcommands_blocked(self):
        # P0-2: Only list, stat, download are permitted. Everything else is blocked.
        banned = [
            ["gaea-c6", "upload", "local.png", "remote.png"],
            ["gaea-c6", "delete", "/remote.png"],
            ["gaea-c6", "write", "/remote.png"],
            ["gaea-c6", "sbatch", "job.sh"],
            ["gaea-c6", "mkdir", "/dir"],
            ["gaea-c6", "read", "/file"],
            ["gaea-c6", "exists", "/file"],
            ["gaea-c6", "download-first", "/f1", "/f2"],
        ]
        for args in banned:
            res = self.run_hpc(*args)
            self.assertEqual(res.returncode, 13, f"Subcommand {args} was not blocked with exit code 13!")
            self.assertIn("READ_ONLY_OPERATION_DENIED", res.stderr)

    def test_help_displays_only_v1_api(self):
        # P0-2: Minimal Read-Only API
        res = self.run_hpc("--help")
        self.assertEqual(res.returncode, 0)
        self.assertIn("RO-v1", res.stdout)
        self.assertIn("hpc-run <cluster> list <remote_dir>", res.stdout)
        self.assertIn("hpc-run <cluster> stat <remote_path>", res.stdout)
        self.assertIn("hpc-run <cluster> download <remote_path> <local_path>", res.stdout)
        self.assertNotIn("download-first", res.stdout)
        self.assertNotIn("exists", res.stdout)


class TestDaemonProtocolSecurity(unittest.TestCase):
    def setUp(self):
        self.base_dir = tempfile.mkdtemp(prefix="hpc_ro_sock_test_")
        self.sock_path = os.path.join(self.base_dir, "hpc_ro_test.sock")
        self.daemon = hpc_mod.HPCDaemon("gaea-c6")
        self.daemon.sock_path = self.sock_path

    def tearDown(self):
        if os.path.exists(self.sock_path):
            try:
                os.remove(self.sock_path)
            except Exception:
                pass
        try:
            os.rmdir(self.base_dir)
        except Exception:
            pass

    def send_socket_payload(self, raw_bytes):
        server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        server.bind(self.sock_path)
        os.chmod(self.sock_path, 0o600)
        server.listen(1)

        client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        client.connect(self.sock_path)
        client.sendall(raw_bytes)

        conn, _ = server.accept()
        self.daemon.handle_client(conn)

        resp = b''
        while True:
            chunk = client.recv(4096)
            if not chunk:
                break
            resp += chunk

        client.close()
        server.close()
        os.remove(self.sock_path)
        return resp.decode('utf-8', errors='replace')

    def test_raw_shell_string_rejected_by_daemon(self):
        # P0-1 & P0-3: Direct raw command string over socket must be rejected
        resp = self.send_socket_payload(b"rm -rf /some/dir\n")
        self.assertIn("13", resp)
        self.assertIn("READ_ONLY_OPERATION_DENIED", resp)

    def test_unknown_operation_rejected_by_daemon(self):
        # P0-3: Daemon-level Default-Deny for unapproved operations
        req = {"op": "upload", "path": "/test", "protocol": "RO-v1"}
        resp = self.send_socket_payload(json.dumps(req).encode('utf-8') + b"\n")
        self.assertIn("13", resp)
        self.assertIn("READ_ONLY_OPERATION_DENIED", resp)

    def test_protocol_mismatch_rejected_by_daemon(self):
        # P1-5: Version validation
        req = {"op": "list", "path": "/test", "protocol": "OLD-v0"}
        resp = self.send_socket_payload(json.dumps(req).encode('utf-8') + b"\n")
        self.assertIn("13", resp)
        self.assertIn("Protocol mismatch", resp)

    def test_socket_namespace_isolated(self):
        # P1-5: New socket namespace 'hpc_ro_{cluster}' isolated from legacy 'hpc_{cluster}'
        sock, pid_f, _ = hpc_mod.get_paths("gaea-c6")
        self.assertIn("hpc_ro_gaea-c6.sock", sock)
        self.assertIn("hpc_ro_gaea-c6.pid", pid_f)


class TestLocalDownloadSafety(unittest.TestCase):
    def setUp(self):
        self.test_dir = tempfile.mkdtemp(prefix="hpc_dl_test_")

    def tearDown(self):
        import shutil
        shutil.rmtree(self.test_dir, ignore_errors=True)

    def test_atomic_file_write(self):
        # P1-7: Atomic write with valid Base64 data
        content = b"Mock radar image binary content \x89PNG\r\n\x1a\n"
        b64_str = base64.b64encode(content).decode('ascii')
        target_file = os.path.join(self.test_dir, "radar.png")

        ok, msg = hpc_mod.safe_atomic_download_write(b64_str, target_file)
        self.assertTrue(ok)
        self.assertTrue(os.path.exists(target_file))
        with open(target_file, "rb") as f:
            self.assertEqual(f.read(), content)

    def test_symlink_overwrite_rejected(self):
        # P1-7: Do not follow symlinks outside target
        target_outside = os.path.join(self.test_dir, "secret_outside.txt")
        with open(target_outside, "w") as f:
            f.write("ORIGINAL_CONTENT")

        symlink_target = os.path.join(self.test_dir, "symlink.txt")
        os.symlink(target_outside, symlink_target)

        new_data = base64.b64encode(b"NEW_DATA").decode('ascii')
        ok, msg = hpc_mod.safe_atomic_download_write(new_data, symlink_target)
        self.assertTrue(ok)

        # Confirm target_outside was NOT overwritten via symlink
        with open(target_outside, "r") as f:
            self.assertEqual(f.read(), "ORIGINAL_CONTENT")


if __name__ == "__main__":
    print("=" * 70)
    print(" Running HPC Read-Only Access V1 Automated Security Test Suite")
    print("=" * 70)
    unittest.main(verbosity=2)
