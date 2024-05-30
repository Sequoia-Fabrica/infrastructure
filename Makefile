.PHONY: ansible

ansible:
	pipenv run ansible-playbook -i ansible/inventory ansible/tend_the_garden.playbook.yml --vault-password-file ~/.sequoia_fabrica_ansible_vault --diff
