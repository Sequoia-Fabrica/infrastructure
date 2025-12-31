# Sequoia Fabrica Infrastructure

Tending to the infrastructure garden of Sequoia Fabrica.

A beloved collection of pets. No cattle here.


                            ___
                        _,-'""   """"`--.
                    ,-'          __,,-- \
                ,'    __,--""""dF      )
                /   .-"Hb_,--""dF      /
                ,'       _Hb ___dF"-._,-'
            ,'      _,-""""   ""--..__
            (     ,-'                  `.
            `._,'     _   _             ;
            ,'     ,' `-'Hb-.___..._,-'
            \    ,'"Hb.-'HH`-.dHF"
                `--'   "Hb  HH  dF"
                        "Hb HH dF
                        "HbHHdF
                        |HHHF
                        |HHH|
                        |HHH|
                        |HHH|
                        |HHH|
                        dHHHb
                        .dFd|bHb.               o
            o       .dHFdH|HbTHb.          o /
        \  Y  |  \__,dHHFdHH|HHhoHHb.______|  Y
        ##########################################

# FAQ
## How do I add a new user?
1. **Add to GitHub Organization**: First, the user must be added to the Sequoia Fabrica GitHub organization. This is required because their SSH public keys will be pulled from their GitHub account.
2. **Update Ansible Configuration**: Add the user to the `sequoia_fabrica_users` list in `ansible/inventory/group_vars/all.yml`
3. **Run Ansible**: Execute `make ansible` to provision the user account on all managed hosts. This will:
    - Create the user account with sudo access
    - Pull their SSH public keys from GitHub
    - Configure their shell environment
    - Set up appropriate permissions

**Note**: The `username` field is the system username that will be created on the servers, while `github_username` is their GitHub account name (used to fetch SSH keys).

## How do I encrypt secret keys?
1. Ask a member of the networking grove for the vault password file. This should not be shared with anyone outside the network grove _ever_.
2. Place the vault password in it's own file in your home directory. _Do not commit_ this file to version control. e.g. `echo "PASSWORD" > ~/.sequoia_fabrica_ansible_vault`
3. Use that file as an argument for `ansible-vault`. Example:
    ```
    ansible-vault encrypt_string --vault-password-file ~/.sequoia_fabrica_ansible_vault 'SECRET_KEY' --name 'authentik_api_token'
    ```
