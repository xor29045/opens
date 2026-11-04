import type { ReactNode } from 'react';
import { Box, Button, Container, Flex, HStack, Link as CLink, Stack, Text } from '@chakra-ui/react';
import { useStore } from '@nanostores/react';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { config } from '@fortawesome/fontawesome-svg-core';
import '@fortawesome/fontawesome-svg-core/styles.css';
import {
  faGauge,
  faBuilding,
  faUsers,
  faCreditCard,
  faFileInvoice,
  faChartLine,
  faShieldHalved,
  faUser,
  faCircleCheck,
  faEye,
  faIdCard,
  type IconDefinition,
} from '@fortawesome/free-solid-svg-icons';
import { sessionAtom, sessionStore, type Role } from '../lib/auth/session';
import { api } from '../lib/api/client';

// Le CSS est importé ci-dessus : on évite l'injection automatique (icônes géantes au premier rendu SSR).
config.autoAddCss = false;

export interface NavItem {
  href: string;
  label: string;
  icon: IconDefinition;
}

const navByRole: Record<Role, NavItem[]> = {
  MANAGER: [
    { href: '/gerant/dashboard', label: 'Vue d’ensemble', icon: faGauge },
    { href: '/gerant/dashboard/etablissement', label: 'Établissement', icon: faBuilding },
    { href: '/gerant/dashboard/profils', label: 'Professionnel·les', icon: faUsers },
    { href: '/gerant/dashboard/abonnement', label: 'Abonnement', icon: faCreditCard },
    { href: '/gerant/dashboard/factures', label: 'Factures', icon: faFileInvoice },
    { href: '/gerant/dashboard/statistiques', label: 'Statistiques', icon: faChartLine },
    { href: '/gerant/dashboard/compte', label: 'Sécurité', icon: faShieldHalved },
  ],
  PROFESSIONAL: [
    { href: '/pro/dashboard', label: 'Mon profil', icon: faUser },
    { href: '/pro/dashboard/statut', label: 'Statut de publication', icon: faCircleCheck },
    { href: '/pro/dashboard/apercu', label: 'Aperçu', icon: faEye },
    { href: '/pro/dashboard/compte', label: 'Sécurité', icon: faShieldHalved },
  ],
  ADMIN: [
    { href: '/admin', label: 'Tableau de bord', icon: faGauge },
    { href: '/admin/etablissements', label: 'Établissements', icon: faBuilding },
    { href: '/admin/profils', label: 'Profils', icon: faIdCard },
    { href: '/admin/utilisateurs', label: 'Utilisateurs', icon: faUsers },
    { href: '/admin/abonnements', label: 'Abonnements', icon: faCreditCard },
  ],
};

export default function AppShell({ role, title, children }: { role: Role; title: string; children: ReactNode }) {
  const session = useStore(sessionAtom);
  const path = typeof window !== 'undefined' ? window.location.pathname : '';
  const items = navByRole[role];

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

          {/* Desktop : menu texte dans le header */}
          <Stack
            as="nav"
            aria-label="Navigation de l’espace"
            direction="row"
            gap="4"
            pb="3"
            wrap="wrap"
            display={{ base: 'none', lg: 'flex' }}
          >
            {items.map((item) => (
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

      <Container as="main" maxW="7xl" pt="6" pb={{ base: '28', lg: '6' }} flex="1" w="full">
        {children}
      </Container>

      {/* Mobile + tablette : barre flottante d’icônes en bas */}
      <Box
        as="nav"
        aria-label="Navigation de l’espace"
        display={{ base: 'block', lg: 'none' }}
        position="fixed"
        zIndex="sticky"
        left="calc(env(safe-area-inset-left, 0px) + 12px)"
        right="calc(env(safe-area-inset-right, 0px) + 12px)"
        bottom="calc(env(safe-area-inset-bottom, 0px) + 12px)"
        mx="auto"
        maxW="md"
        bg="bg"
        borderWidth="1px"
        borderColor="border"
        borderRadius="2xl"
        boxShadow="lg"
        p="1.5"
      >
        <HStack gap="1" justify="space-around">
          {items.map((item) => {
            const active = path === item.href;
            return (
              <CLink
                key={item.href}
                href={item.href}
                aria-label={item.label}
                title={item.label}
                aria-current={active ? 'page' : undefined}
                flex="1"
                minW="11"
                h="11"
                display="flex"
                alignItems="center"
                justifyContent="center"
                borderRadius="xl"
                fontSize="lg"
                textDecoration="none"
                color={active ? 'accent.fg' : 'fg.muted'}
                bg={active ? 'bg.muted' : 'transparent'}
                _hover={{ textDecoration: 'none', bg: 'bg.muted' }}
              >
                <FontAwesomeIcon icon={item.icon} fixedWidth aria-hidden="true" />
              </CLink>
            );
          })}
        </HStack>
      </Box>
    </Flex>
  );
}
