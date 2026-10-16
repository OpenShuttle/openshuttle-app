# Appli de tournois de badminton — cadrage v1 pour Claude Design

## Vision et périmètre v1

Une application open source pour gérer de bout en bout un tournoi de badminton de club non homologué, entièrement sur un PC, sans aucune connexion. Elle s'inspire de ShuttleCup mais se distingue par un modèle de formats composé de phases librement enchaînées, et par une table de marque pensée pour corriger le tournoi en direct.

**Dans la v1**

- Application de bureau 100 % locale : un PC de table de marque et un second écran pour la salle, sans réseau ni impression.
- Une ou deux catégories de rencontre par tournoi (par exemple double homme et double dame), sans joueur commun, sur des terrains partagés.
- Formats paramétrables par phases, avec une condition de victoire propre à chaque phase.
- Presets réutilisables, tournoi préparé en amont et modifiable le jour même.
- Pointage, tirage, gestion des terrains et appel des matchs.
- Saisie des scores à la table, en partant du terrain concerné.
- Manipulation manuelle des poules et tableaux en cours de tournoi, avec annulation et historique.
- Classement intégral à la fin.
- Interface bilingue français et anglais dès la v1.

**Hors v1** : tout ce qui passe par le réseau (suivi et saisie sur téléphone, publication en ligne), plus de deux catégories simultanées, un même joueur inscrit dans deux catégories.

## Personas

Trois profils utilisent l'application, mais seuls les deux premiers la manipulent. Le public est souvent bénévole, parfois peu à l'aise avec l'informatique, et travaille dans un gymnase bruyant et sous pression.

| Persona | Contexte | Besoin principal | Ce qui le met en difficulté |
| --- | --- | --- | --- |
| Organisateur | Chez lui, les jours précédents, au calme | Préparer le tournoi : inscrits, format, terrains, horaires | Un format trop rigide, devoir tout ressaisir d'un tournoi à l'autre |
| Table de marque | Le jour J, au gymnase, souvent 2 personnes sur 1 PC | Faire tourner les terrains, saisir les scores, gérer les imprévus | Une action bloquée par l'outil, une erreur impossible à annuler |
| Joueurs et public | Dans la salle, sans manipuler l'appli | Savoir quand et sur quel terrain ils jouent, suivre poules et tableaux | Un écran illisible à 10 m, un appel raté |

L'organisateur et la table de marque sont souvent la même personne à deux moments différents. Le public, lui, ne voit que l'écran d'appel.

## Glossaire

Ces termes doivent être utilisés tels quels dans toute l'interface.

| Terme | Anglais (code et UI) | Définition |
| --- | --- | --- |
| Tournoi | Tournament | Un événement d'une journée (ou plus) pour une ou deux catégories de rencontre, avec ses inscrits, ses terrains et un format par catégorie |
| Catégorie | Category | Le type de rencontre : simple homme, simple dame, double homme, double dame, double mixte. Sigle affiché partout : SH, SD, DH, DD, DM |
| Participant | Entrant | Un joueur en simple, une paire en double |
| Dossard | Bib number | Numéro attribué à chaque participant au pointage, affiché partout à côté du nom |
| Format | Format | L'enchaînement des phases d'une catégorie |
| Preset | Preset | Un format sauvegardé pour être réutilisé sur un autre tournoi |
| Phase | Stage | Une étape du format : un ensemble de poules ou un tableau |
| Poule | Pool | Groupe de participants qui se rencontrent tous, avec un classement interne |
| Tableau | Bracket | Arbre à élimination directe (principal, consolante, places 5-8…) |
| Tableau consolante | Consolation bracket | Tableau des participants non qualifiés pour le principal |
| Routage | Advancement rule | Règle qui envoie les participants d'une phase vers la suivante selon leur rang |
| Condition de victoire | Win condition | Comment se gagne un match dans une phase : sets, points, plafond, ou temps |
| Placement complet | Full placement | Tableau où chaque place est jouée, pour obtenir un classement sans ex aequo |
| Point en or | Golden point | Point unique décisif joué en cas d'égalité à la fin d'un match au temps |
| Rotation | Round (timed) | Série de matchs au temps lancés et sifflés ensemble sur tous les terrains |
| Score en attente | Pending score | Match terminé dont le score n'est pas encore saisi ; le terrain reste utilisable |
| Priorité des terrains | Court priority | Règle qui départage deux catégories pour l'attribution des terrains libres |
| WO | Walkover | Match gagné par forfait de l'adversaire, sans score |
| Pointage | Check-in | Validation de la présence des inscrits le jour J |
| Tirage | Draw | Répartition des participants dans les poules |
| Tête de série | Seed | Participant placé pour ne pas croiser trop tôt les autres favoris |
| Table de marque | Control desk | Le poste qui pilote le tournoi le jour J |
| Appel | Match call | Annonce d'un match sur un terrain, affichée sur l'écran d'appel |
| Écran d'appel | Display screen | Seconde fenêtre, sur l'écran ou le vidéoprojecteur de la salle |

## Modèle de tournoi

Un format est un graphe de phases. Chaque phase a ses participants et sa condition de victoire, et des règles de routage envoient les participants vers la phase suivante selon leur rang.

**Exemple de format (16 participants)**

1. Phase 1 : 4 poules de 4, tous contre tous, match au temps de 12 min avec point en or.
2. Rangs 1 et 2 de chaque poule → Tableau principal (8 participants, placement complet, 2 sets gagnants de 21) → places 1 à 8.
3. Rangs 3 et 4 de chaque poule → Tableau consolante (8 participants, placement complet, 1 set de 21, plafond 30) → places 9 à 16.
4. Les deux tableaux mis bout à bout donnent le classement intégral, places 1 à 16.

Chaque phase porte sa propre condition de victoire, et le classement intégral s'obtient en mettant bout à bout les places jouées dans chaque tableau.

**Paramètres d'une phase**

| Paramètre | Valeurs possibles |
| --- | --- |
| Type | Poules (tous contre tous) ou tableau à élimination directe |
| Taille | Nombre et taille des poules, ou taille du tableau |
| Condition de victoire | Sets gagnants, points par set, plafond de points, ou match au temps (durée + point en or) |
| Départage (poules) | Ordre des critères, voir Règles métier |
| Placement (tableaux) | Serpentin, croisement des poules, séparation des participants d'une même poule au 1er tour |
| Classement (tableaux) | Placement complet (chaque place est jouée) ou ex aequo par tour perdu |
| Routage | Pour chaque rang, la phase de destination |

**Classement intégral** : il n'est complet que si chaque tableau est à placement complet (les perdants des quarts jouent les places 5 à 8, etc.). Sinon, les perdants d'un même tour sont classés ex aequo.

**Un format par catégorie** : avec deux catégories, chacune a son propre format, indépendant (nombre de poules, condition de victoire, tableaux).

**Presets** : un preset est un graphe de phases enregistré. L'éditeur doit permettre de le prévisualiser avec un nombre donné de participants (combien de matchs, combien de tours) avant de l'appliquer.

## Règles métier

Ces règles sont arrêtées pour la v1 et conditionnent plusieurs écrans.

**Forfaits**

- **En poule** : un forfait passe tous les matchs du participant en WO, joués ou non. La victoire compte pour l'adversaire, mais ces matchs sont exclus des calculs de sets et de points. Le participant forfait finit dernier de sa poule.
- **En tableau** : un forfait vaut défaite, l'adversaire passe au tour suivant.
- L'interface affiche « WO » à la place du score.

**Départage en poule**

Critères appliqués dans l'ordre, ordre paramétrable par phase :

1. Nombre de victoires
2. Confrontation directe (entre deux ex aequo)
3. Différence de sets
4. Différence de points
5. Tirage au sort

Les critères utilisent des différences et jamais des ratios, pour que les WO restent neutres.

**Match au temps**

- Le vainqueur est celui qui mène au coup de sifflet. En cas d'égalité, un point en or décide.
- Une phase au temps se joue par **rotations** : tous les terrains commencent et finissent en même temps. La table lance la rotation, puis siffle la fin ; le décompte n'est qu'un repère et rien ne change d'état sans elle.
- À la fin d'une rotation, tous les matchs passent en score en attente. La rotation suivante peut être lancée sans attendre la saisie.
- Un participant ne joue qu'une fois par rotation ; ceux qui viennent de se reposer passent en priorité. Le temps de repos minimum ne s'applique pas.
- Le match au temps n'est disponible que si le tournoi n'a qu'une seule catégorie.

**Fin d'un match (sets et points)**

- Par défaut, c'est la table de marque qui constate la fin d'un match (bouton « Terminer » sur le terrain, ou saisie directe du score).
- Un terrain terminé peut recevoir le match suivant avant la saisie du score : le match fini passe dans la liste des scores en attente. Le temps de repos part de la fin réelle du match, pas de la saisie.

**Deux catégories simultanées**

- Deux catégories maximum, sans joueur commun.
- Les terrains sont partagés. Une priorité des terrains, réglable à tout moment, choisit le prochain match : équilibrée (la catégorie la moins avancée d'abord) ou une catégorie d'abord.
- Chaque catégorie a son tirage, ses poules, ses tableaux et son classement intégral.

**Manipulation manuelle**

La table de marque peut à tout moment déplacer un participant, modifier ou effacer un score, inverser deux matchs, déclarer un forfait. Chaque action est enregistrée dans un journal : elle peut être annulée et l'historique indique qui a changé quoi et quand. L'outil avertit des incohérences mais ne bloque jamais une action.

## Déroulé d'un tournoi

Un tournoi suit six étapes, et chacune reste modifiable jusqu'à la clôture.

1. **Préparation (en amont)** : l'organisateur crée le tournoi, choisit une ou deux catégories, importe ou saisit les inscrits, part d'un preset ou compose le format, définit le nombre de terrains et les horaires.
2. **Pointage (jour J, arrivée)** : la table coche les présents joueur par joueur (un joueur pointé une fois l'est pour toutes ses inscriptions), attribue les dossards, retire les absents et ajoute les inscrits de dernière minute. Une paire est prête quand ses deux joueurs sont pointés.
3. **Tirage** : génération des poules de chaque catégorie à partir des seuls présents, avec têtes de série et séparation par club. La table peut ajuster à la main avant de valider.
4. **Matchs** : l'application propose le prochain match pour chaque terrain libre ou terminé, en tenant compte du temps de repos et de la priorité des terrains. La table l'appelle, il s'affiche sur l'écran d'appel. À la fin, la table termine le match, puis saisit le score tout de suite ou plus tard depuis la liste des scores en attente. En phase au temps, la table lance et siffle des rotations entières.
5. **Passage de phase** : quand une poule est terminée, le routage remplit les tableaux suivants. La table vérifie et peut corriger avant de lancer.
6. **Clôture** : classement intégral, export, archivage du fichier tournoi sur le PC.

## Écrans à designer

À designer en priorité : le tableau de bord de la table de marque, l'écran d'appel et l'éditeur de format. La direction visuelle est libre, tant qu'elle reste sobre et très lisible.

**Navigation**

- **Hors tournoi**, un accueil : tournois récents (un fichier par tournoi), « Nouveau tournoi », « Ouvrir un fichier », puis accès au registre des joueurs, aux presets et aux paramètres.
- **Dans un tournoi**, une navigation latérale qui suit le déroulé en deux groupes : Préparation (Infos, Inscrits, Format) puis Jour J (Pointage, Tirage, Table de marque, Phases, Résultats). L'étape en cours est mise en avant, les étapes terminées sont cochées, tout reste cliquable.
- **Barre haute permanente** : nom et catégories du tournoi, heure, dernière action avec un bouton « Annuler », pilotage de l'écran d'appel (ouvrir, choisir le moniteur).

**Tableau de bord de la table de marque**

L'écran où la table passe 90 % du jour J. Deux zones côte à côte :

- **Grille des terrains** (une carte par terrain, sigle de catégorie sur chaque match) avec trois états distincts :
  - en cours : participants, poule, temps joué, bouton « Terminer » ;
  - libre : prochain match proposé et bouton « Appeler » ;
  - terminé : le match fini (cliquable pour saisir le score) et le match suivant proposé, avec « Saisir le score » et « Lancer le suivant ».
- **En phase au temps**, un bandeau de rotation au-dessus de la grille : décompte commun, « Siffler la fin », puis « Lancer la rotation suivante ». Pas de décompte par terrain.
- **Colonne de droite**, de haut en bas : la liste des scores en attente (terrain, match, heure de fin, mise en évidence après 10 min), la file des prochains matchs avec un filtre par catégorie et la raison quand un match n'est pas prêt (joueur en repos depuis X min, joueur occupé sur un autre terrain), puis les alertes qui demandent une décision (poule terminée, tableaux à valider).
- **Priorité des terrains** réglable directement depuis le tableau de bord.

La saisie de score n'est pas un écran à part : un panneau s'ouvre au clic sur un terrain, avec les deux camps, autant de champs de set que la condition de victoire en prévoit, les boutons « WO » et « Point en or » quand ils s'appliquent, et une validation au clavier.

**Liste des écrans**

| # | Écran | Persona | Contenu | Actions clés |
| --- | --- | --- | --- | --- |
| 1 | Accueil | Organisateur | Tournois récents, accès registre, presets, paramètres | Créer, ouvrir un fichier, dupliquer |
| 2 | Registre des joueurs | Organisateur | Joueurs et clubs réutilisables d'un tournoi à l'autre | Ajouter, modifier, importer en CSV |
| 3 | Infos du tournoi | Organisateur | Nom, date, lieu, une ou deux catégories, priorité des terrains, nombre de terrains, horaires, temps de repos minimum, estimation de l'heure de fin | Modifier, partir d'un preset |
| 4 | Inscrits | Organisateur | Participants (joueur ou paire) par catégorie, club, niveau | Ajouter depuis le registre, former les paires |
| 5 | Format | Organisateur | Un onglet par catégorie ; au centre le graphe des phases, à droite les réglages de la phase sélectionnée, en bas l'aperçu pour N participants (matchs, tours) | Ajouter et relier des phases, paramétrer, enregistrer comme preset |
| 6 | Presets | Organisateur | Presets enregistrés avec un aperçu du graphe, ouverts dans l'éditeur de format | Appliquer, dupliquer, supprimer |
| 7 | Pointage | Table de marque | Grande liste par joueur avec recherche, catégorie, partenaire, dossard, compteurs « 40 / 42 joueurs · 20 / 21 paires complètes » | Marquer présent en un clic, ajouter sur place |
| 8 | Tirage | Table de marque | Une section par catégorie, une colonne par poule, cartes de participants, badges d'alerte (même club) | Glisser-déposer, relancer, valider |
| 9 | Table de marque | Table de marque | Tableau de bord décrit ci-dessus, avec le panneau de saisie | Appeler, terminer, lancer le suivant, saisir un score, déclarer un forfait, lancer et siffler une rotation |
| 10 | Phases | Table de marque | Choix de la catégorie, puis un onglet par phase : classements de poules ou arbre du tableau | Mode « Correction » à activer pour déplacer, inverser, modifier, volet historique |
| 11 | Résultats | Table de marque | Classement intégral par catégorie | Exporter, archiver |
| 12 | Écran d'appel | Joueurs et public | Seconde fenêtre plein écran, catégories mélangées avec un sigle très visible : matchs appelés en très grand avec le terrain, grille des terrains en cours, rotation automatique entre poules et tableaux. En phase au temps : la liste des terrains de la rotation et un grand décompte commun | Aucune, affichage seul |
| 13 | Paramètres | Organisateur | Langue, moniteur de l'écran d'appel, valeurs par défaut | Modifier |

## Principes UX et contraintes matérielles

L'interface doit rester utilisable par un bénévole stressé, sur un PC portable, dans un gymnase.

**Matériel**

- Un PC portable (écran 13 à 15 pouces), clavier et souris, sans tactile garanti.
- Un second écran ou vidéoprojecteur en HDMI, lu à 10 m ou plus.
- Aucun réseau, ni local ni internet, et aucune impression.
- Application de bureau (Wails) dont l'interface est en web (Angular) : deux fenêtres natives, la principale et l'écran d'appel.

**Principes**

- **Ne jamais bloquer** : l'outil avertit, la table décide. Toute action est annulable.
- **Le tableau de bord d'abord** : le jour J, tout part de la table de marque, et les actions fréquentes s'y font en un ou deux clics.
- **L'état d'un terrain se lit d'un coup d'œil** : en cours, libre, terminé, distincts par la couleur et le libellé.
- **La catégorie se lit d'un coup d'œil** : sigle coloré (DH, DD…) à côté de chaque match, partout.
- **Sobriété visuelle** : fonds et neutres en gris froids et blanc ; la couleur est réservée aux états (en cours, libre, terminé, alerte) et aux catégories.
- **Lisible de loin** : l'écran d'appel utilise de très grands caractères, un fort contraste et le numéro de terrain en évidence.
- **Dossard + nom** partout, pour éviter les homonymes.
- **Vocabulaire du glossaire** uniquement.
- **Bilingue** : français et anglais. Le français est 15 à 20 % plus long, les boutons et libellés doivent le supporter.
- **Raccourcis clavier** pour la saisie de score et l'appel des matchs : chiffre du terrain (appeler, terminer ou saisir), S (plus ancien score en attente), L / F (lancer / siffler la rotation), Entrée, Échap, Ctrl Z. Compatibles AZERTY.

## Hors périmètre v1 et pistes v2

La v2 ajoute un mode en ligne optionnel, sans jamais rendre le tournoi dépendant du réseau. À ne pas designer pour la v1, mais à garder en tête pour ne pas fermer de portes.

- **Local d'abord** : le PC de la table de marque reste la source de vérité. Si internet tombe, le tournoi continue et seul le suivi en ligne se fige.
- **Publication à sens unique** : le PC pousse une vue publique (poules, tableaux, scores, appels) vers un relais cloud hébergé en UE.
- **Suivi et saisie sur téléphone** : accès par QR code, avec la 4G des spectateurs ou un réseau local optionnel.
- **Pseudonymisation** : sur le cloud, aucun nom réel. Au choix par tournoi : dossard, pseudo choisi au pointage, nom d'équipe aléatoire, ou prénom + initiale.
- **Purge automatique** du tournoi en ligne 7 jours après sa fin.
- **Plus de deux catégories simultanées**, et un même joueur inscrit dans plusieurs catégories (gestion des conflits et du repos entre catégories).
