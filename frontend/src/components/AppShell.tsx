import type { ReactNode } from 'react';
import { Box, Button, Container, Flex, HStack, Link as CLink, Stack, Text } from '@chakra-ui/react';
import { useStore } from '@nanostores/react';
import { sessionAtom, sessionStore, type Role } from '../lib/auth/session';
import { api } from '../lib/api/client';

export interface NavItem {
  href: string;
  label: string;
}

const navByRole: Record<Role, NavItem[]> = {
  MANAGER: [
    { href: '/gerant/dashboard', label: 'Vue d’ensemble' },
    { href: '/gerant/dashboard/etablissement', label: 'Établissement' },
    { href: '/gerant/dashboard/profils', label: 'Professionnel·les' },
    { href: '/gerant/dashboard/abonnement', label: 'Abonnement' },
    { href: '/gerant/dashboard/factures', label: 'Factures' },
    { href: '/gerant/dashboard/statistiques', label: 'Statistiques' },
    { href: '/gerant/dashboard/compte', label: 'Sécurité' },
  ],
  PROFESSIONAL: [
    { href: '/pro/dashboard', label: 'Mon profil' },
    { href: '/pro/dashboard/statut', label: 'Statut de publication' },
    { href: '/pro/dashboard/apercu', label: 'Aperçu' },
    { href: '/pro/dashboard/compte', label: 'Sécurité' },
  ],
  CLIENT: [
    { href: '/client/espace/decouvrir', label: 'Découvrir' },
    { href: '/client/espace', label: 'Favoris' },
    { href: '/client/espace/compte', label: 'Mon compte' },
  ],
  ADMIN: [
    { href: '/admin', label: 'Tableau de bord' },
    { href: '/admin/etablissements', label: 'Établissements' },
    { href: '/admin/profils', label: 'Profils' },
    { href: '/admin/utilisateurs', label: 'Utilisateurs' },
    { href: '/admin/abonnements', label: 'Abonnements' },
    { href: '/admin/sessions', label: 'Sessions' },
  ],
};

export default function AppShell({ role, title, children }: { role: Role; title: string; children: ReactNode }) {
  const session = useStore(sessionAtom);
  const path = typeof window !== 'undefined' ? window.location.pathname : '';
  const logout = async () => {
    try {
      await api('/auth/logout', { method: 'POST', auth: true });
    } catch {
      // The session is dropped locally in every case.
    }
    sessionStore.clear();
    window.location.replace('/');
  };
  return (
    <Flex minH="100vh" direction="column">
      <Box as="header" borderBottomWidth="1px" borderColor="border" bg="bg">
        <Container maxW="7xl">
          <HStack justify="space-between" py="3" wrap="wrap" gap="3">
            <HStack gap="3">
              <CLink href="/" fontWeight="bold">Opensex</CLink>
              <Text color="fg.muted" fontSize="sm">/ {title}</Text>
            </HStack>
            <HStack gap="3">
              <Text fontSize="sm" color="fg.muted">{session.user?.username}</Text>
              <Button size="sm" variant="outline" onClick={logout}>Se déconnecter</Button>
            </HStack>
          </HStack>
          <Stack as="nav" aria-label="Navigation de l’espace" direction="row" gap="4" pb="3" wrap="wrap">
            {navByRole[role].map((item) => (
              <CLink
                key={item.href}
                href={item.href}
                fontSize="sm"
                fontWeight={path === item.href ? 'semibold' : 'normal'}
                color={path === item.href ? 'accent.fg' : 'fg.muted'}
                aria-current={path === item.href ? 'page' : undefined}
              >
                {item.label}
              </CLink>
            ))}
          </Stack>
        </Container>
      </Box>
      <Container as="main" maxW="7xl" py="6" flex="1" w="full">
        {children}
      </Container>
    </Flex>
  );
}
