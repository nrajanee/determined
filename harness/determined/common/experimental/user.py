import uuid
from typing import Any, Dict, List, Optional

from requests import Response

from determined.common import api
from determined.common.api import authentication, bindings


# All the creates should be in
class AgentUserGroup:
    def __init__(
        self,
        agent_uid: Optional[int],
        agent_gid: Optional[int],
        agent_user: Optional[str],
        agent_group: Optional[str],
    ):
        self.agent_uid = agent_uid
        self.agent_gid = agent_gid
        self.agent_user = agent_user
        self.agent_group = agent_group


class User:
    def __init__(
        self,
        username: str,
        password: str,
        admin: bool,
        session: api.Session,
    ):
        self.username = username
        self.password = password
        self.admin = admin
        self.active = True
        self.agent_uid = None
        self.agent_gid = None
        self.agent_user = None
        self.agent_group = None
        self.session = session

    def update_user(
        username: str,
        active: Optional[bool] = None,
        password: Optional[str] = None,
        agent_user_group: Optional[AgentUserGroup] = None,
    ) -> Response:
        # new API -> bindings.patch_PatchUser(user_id, patchUser)
        # return API response
        pass

    def update_user(
        user_id: str,
        active: Optional[bool] = None,
        password: Optional[str] = None,
        agent_user_group: Optional[AgentUserGroup] = None,
    ) -> Response:
        # new API -> bindings.patch_PatchUser(user_id, patchUser)
        # edit above API for bindings.patch_PatchUser(username, patchUser)
        # return API response
        pass
     
    def update_username(current_username: str, new_username: str) -> Response:
        # return API response

        # can use the edited API patch_PatchUser(username, patchUser) API (need to also add username to message PatchUser in user.proto)

        pass

    def activate_user(username: str) -> None:
        # calls update_user with active = true
        # can use the edited API patch_PatchUser(username, patchUser) API
        pass

    def activate_user(user_id: int) -> None:
        # calls update_user with active = true
        # new API -> bindings.patch_PatchUser(user_id, patchUser)
        pass

    def deactivate_user(username: str) -> None:
        # calls update_user with active = false
        # can use the edited API patch_PatchUser(username, patchUser) API
        pass

    def deactivate_user(user_id: int) -> None:
        # calls update_user with active = false
        # can use the edited API patch_PatchUser(user_id, patchUser) API
        pass

    def log_in_user(username: str, password: str) -> None:
        # for password should they pass in plain text or hashed value (applies to other methods too.)
        #  but how would we unhash it?

        pass

    def log_out_user(username: str) -> None:
        pass

    def change_password(username: str, new_password: str) -> None:
        # can also get user from authentication.must_cli_auth().get_session_user()
        # API bindings.patch_PatchUser (add username part) can't change the password. Should I edit this (need to also add username to message PatchUser in user.proto) or add new API method in api_user.go
        pass

    def change_password(user_id: str, new_password: str) -> None:
        # can also get user from authentication.must_cli_auth().get_session_user()
        # API bindings.patch_PatchUser can't change the password. Should I edit this (need to also add username to message PatchUser in user.proto) or add new API method in api_user.go
        pass

    def link_with_agent_user(username: str, agent_user_group: AgentUserGroup) -> None:
        # calls update user with these args wrapped in agent_user_group.
        pass

    def link_with_agent_user(user_id: int, agent_user_group: AgentUserGroup) -> None:
        # calls update user with these args wrapped in agent_user_group.
        pass

    def whoami() -> str:
        # return username
        # need to return curr_user username. 
        pass
