import subprocess

import pytest

from determined.common.api import authentication, bindings, certs
from determined.common.experimental import session
from tests import config as conf
from tests import experiment as exp


@pytest.mark.e2e_cpu
def test_support_bundle():
    trial_id = exp.create_exp_get_trial_id()

    command = ["det", "trial", "support-bundle", str(trial_id), "-o", f"e2etest_trial{trial_id}"]

    completed_process = subprocess.run(
        command, universal_newlines=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )

    assert completed_process.returncode == 0, "\nstdout:\n{} \nstderr:\n{}".format(
        completed_process.stdout, completed_process.stderr
    )
