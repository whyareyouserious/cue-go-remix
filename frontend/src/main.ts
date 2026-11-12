import { bindInteractions } from './app/interactions';
import { observeLayout } from './app/observe';
import { mountShell } from './app/shell';
import { start } from './app/start';
import './styles/index.css';

mountShell();
observeLayout();
bindInteractions();
void start();
